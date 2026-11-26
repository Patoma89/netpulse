package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"netpulse/internal/ping"
)

const (
	defaultIdleDuration     = 3 * time.Second
	defaultLoadDuration     = 5 * time.Second
	defaultBloatPingRate    = 150 * time.Millisecond
	defaultLoadConcurrency  = 4
	defaultUploadChunkBytes = 4 * 1024 * 1024
)

var (
	// ErrBufferbloatRunning is returned if a second test is started while one is already active.
	ErrBufferbloatRunning = errors.New("bufferbloat test already running")

	defaultDownloadURLs = []string{
		"https://speed.cloudflare.com/__down?bytes=25000000",
		"https://proof.ovh.net/files/10Mb.dat",
	}
	defaultUploadURL = "https://speed.cloudflare.com/__up"
)

// LoadFunc saturates download or upload bandwidth until ctx expires, reporting transferred bytes.
type LoadFunc func(ctx context.Context, onBytes func(int64)) error

// BufferbloatProgress streams real-time test progress to the frontend.
type BufferbloatProgress struct {
	Phase         string  `json:"phase"` // "idle" | "download" | "upload" | "done" | "cancelled" | "error"
	ProgressPct   int     `json:"progressPct"`
	CurrentPingMs float64 `json:"currentPingMs"`
	CurrentMbps   float64 `json:"currentMbps"`
	Message       string  `json:"message"`
}

// BufferbloatResult holds the final latency-under-load measurements and A+..F grade.
type BufferbloatResult struct {
	Grade            string  `json:"grade"`
	Severity         string  `json:"severity"`
	Summary          string  `json:"summary"`
	Recommendation   string  `json:"recommendation"`
	Target           string  `json:"target"`
	IdleMedianMs     float64 `json:"idleMedianMs"`
	IdleAvgMs        float64 `json:"idleAvgMs"`
	DownloadMedianMs float64 `json:"downloadMedianMs"`
	DownloadDeltaMs  float64 `json:"downloadDeltaMs"`
	DownloadMbps     float64 `json:"downloadMbps"`
	DownloadLossPct  float64 `json:"downloadLossPct"`
	UploadMedianMs   float64 `json:"uploadMedianMs"`
	UploadDeltaMs    float64 `json:"uploadDeltaMs"`
	UploadMbps       float64 `json:"uploadMbps"`
	UploadLossPct    float64 `json:"uploadLossPct"`
	UploadSkipped    bool    `json:"uploadSkipped"`
	CompletedAt      string  `json:"completedAt"`
}

// GradeBufferbloat computes the Waveform-standard grade (A+ to F) from latency deltas and packet loss.
func GradeBufferbloat(downloadDeltaMs, uploadDeltaMs, worstLossPct float64) (grade, severity, summary, recommendation string) {
	maxDelta := math.Max(0, math.Max(downloadDeltaMs, uploadDeltaMs))

	switch {
	case maxDelta < 5:
		grade = "A+"
		severity = "emerald"
		summary = "Latencia prácticamente inmune a carga pesada."
		recommendation = "Tu router gestiona colas de tráfico de forma excelente (SQM/FQ-CoDel o ancho de banda amplio). No necesitás ajustes extra."
	case maxDelta < 30:
		grade = "A"
		severity = "emerald"
		summary = "Excelente control de cola bajo descarga y subida."
		recommendation = "Muy buen desempeño para gaming competitivo y llamadas incluso mientras otros descargan."
	case maxDelta < 60:
		grade = "B"
		severity = "cyan"
		summary = "Bufferbloat leve bajo saturación."
		recommendation = "Rendimiento sólido en uso normal. Si jugás mientras alguien sube archivos pesados, activar QoS/SQM al 85-90% de tu ancho de banda lo deja en A+."
	case maxDelta < 200:
		grade = "C"
		severity = "amber"
		summary = "Bufferbloat moderado: el ping se infla cuando saturan tu conexión."
		recommendation = "Activá Smart Queue Management (SQM / Cake / FQ-CoDel) o limitá el ancho de banda de descargas/torrents al 85% de tu línea."
	case maxDelta < 400:
		grade = "D"
		severity = "amber"
		summary = "Bufferbloat alto: descargas o backups disparan picos fuertes de lag."
		recommendation = "Configurá QoS/SQM en tu router limitando subida y bajada al ~85% de tu velocidad real, y evitá subir archivos en segundo plano al jugar."
	default:
		grade = "F"
		severity = "rose"
		summary = "Bufferbloat severo: las colas del módem/router colapsan bajo carga."
		recommendation = "Urgente: activá SQM (Cake/FQ-CoDel) si tu router lo soporta, o aplicá límites de ancho de banda en clientes de descarga/cloud."
	}

	if worstLossPct >= 15 {
		grade = "F"
		severity = "rose"
		summary = fmt.Sprintf("Pérdida severa de paquetes bajo carga (%.1f%%).", worstLossPct)
		recommendation = "Además de bufferbloat, se pierden paquetes bajo tráfico intenso. Revisá cableado, saturación de subida o estabilidad del módem ISP."
	} else if worstLossPct >= 5 && (grade == "A+" || grade == "A" || grade == "B") {
		grade = "C"
		severity = "amber"
		summary = fmt.Sprintf("Se detectó %.1f%% de pérdida de paquetes bajo carga.", worstLossPct)
		recommendation = "Aunque la mediana subió poco, hubo paquetes descartados bajo carga. Limitá la velocidad de subida al 85%."
	}

	return grade, severity, summary, recommendation
}

// BufferbloatRunner orchestrates the 3-phase (Idle, Download, Upload) latency-under-load test.
type BufferbloatRunner struct {
	mu           sync.Mutex
	pinger       ping.Pinger
	idleDur      time.Duration
	downloadDur  time.Duration
	uploadDur    time.Duration
	pingInterval time.Duration
	downloadLoad LoadFunc
	uploadLoad   LoadFunc
	running      bool
	cancelFunc   context.CancelFunc
	lastResult   *BufferbloatResult
}

// NewBufferbloatRunner builds a BufferbloatRunner with default HTTP load generators.
func NewBufferbloatRunner(pinger ping.Pinger) *BufferbloatRunner {
	return &BufferbloatRunner{
		pinger:       pinger,
		idleDur:      defaultIdleDuration,
		downloadDur:  defaultLoadDuration,
		uploadDur:    defaultLoadDuration,
		pingInterval: defaultBloatPingRate,
		downloadLoad: defaultHTTPDownloadLoad,
		uploadLoad:   defaultHTTPUploadLoad,
	}
}

// LastResult returns a copy of the most recent completed BufferbloatResult, if any.
func (r *BufferbloatRunner) LastResult() *BufferbloatResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastResult == nil {
		return nil
	}
	copyVal := *r.lastResult
	return &copyVal
}

// Cancel aborts an in-flight bufferbloat test.
func (r *BufferbloatRunner) Cancel() {
	r.mu.Lock()
	cancel := r.cancelFunc
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Run executes Idle -> Download -> Upload phases while pinging target and emitting progress.
func (r *BufferbloatRunner) Run(ctx context.Context, target string, onProgress func(BufferbloatProgress)) (BufferbloatResult, error) {
	if target == "" {
		target = "1.1.1.1"
	}

	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return BufferbloatResult{}, ErrBufferbloatRunning
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.running = true
	r.cancelFunc = cancel
	idleDur := r.idleDur
	dlDur := r.downloadDur
	ulDur := r.uploadDur
	pingInterval := r.pingInterval
	dlLoad := r.downloadLoad
	ulLoad := r.uploadLoad
	r.mu.Unlock()

	defer func() {
		cancel()
		r.mu.Lock()
		r.running = false
		r.cancelFunc = nil
		r.mu.Unlock()
	}()

	emit := func(p BufferbloatProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	// Phase 1: Idle baseline
	idleSamples, idleLoss, err := r.measurePhase(runCtx, "idle", target, idleDur, pingInterval, nil, 0, 25, "Midiendo latencia base en reposo...", emit)
	if err != nil {
		emitCancelledOrError(runCtx, err, emit)
		return BufferbloatResult{}, err
	}
	if len(idleSamples) == 0 {
		err := errors.New("no se recibieron respuestas ICMP en reposo")
		emit(BufferbloatProgress{Phase: "error", ProgressPct: 0, Message: err.Error()})
		return BufferbloatResult{}, err
	}
	_ = idleLoss
	idleMedian := medianFloat64(idleSamples)
	idleAvg := meanFloat64(idleSamples)

	// Phase 2: Download saturation
	dlSamples, dlLoss, dlMbps, dlErr := r.measureLoadPhase(runCtx, "download", target, dlDur, pingInterval, dlLoad, 25, 62, "Saturando descarga y midiendo latencia...", emit)
	if dlErr != nil {
		emitCancelledOrError(runCtx, dlErr, emit)
		return BufferbloatResult{}, dlErr
	}
	dlMedian := idleMedian
	if len(dlSamples) > 0 {
		dlMedian = medianFloat64(dlSamples)
	}
	dlDelta := round1(math.Max(0, dlMedian-idleMedian))

	// Phase 3: Upload saturation (degrades gracefully if blocked by local firewall)
	ulSamples, ulLoss, ulMbps, ulErr := r.measureLoadPhase(runCtx, "upload", target, ulDur, pingInterval, ulLoad, 62, 98, "Saturando subida y midiendo latencia...", emit)
	if ulErr != nil && runCtx.Err() != nil {
		emitCancelledOrError(runCtx, runCtx.Err(), emit)
		return BufferbloatResult{}, runCtx.Err()
	}

	uploadSkipped := false
	ulMedian := idleMedian
	ulDelta := 0.0
	if ulErr != nil || ulMbps <= 0.01 {
		uploadSkipped = true
		ulMedian = idleMedian
		ulDelta = 0
		ulLoss = 0
	} else {
		if len(ulSamples) > 0 {
			ulMedian = medianFloat64(ulSamples)
		}
		ulDelta = round1(math.Max(0, ulMedian-idleMedian))
	}

	worstLoss := math.Max(dlLoss, ulLoss)
	grade, severity, summary, rec := GradeBufferbloat(dlDelta, ulDelta, worstLoss)

	result := BufferbloatResult{
		Grade:            grade,
		Severity:         severity,
		Summary:          summary,
		Recommendation:   rec,
		Target:           target,
		IdleMedianMs:     round1(idleMedian),
		IdleAvgMs:        round1(idleAvg),
		DownloadMedianMs: round1(dlMedian),
		DownloadDeltaMs:  dlDelta,
		DownloadMbps:     round1(dlMbps),
		DownloadLossPct:  round1(dlLoss),
		UploadMedianMs:   round1(ulMedian),
		UploadDeltaMs:    ulDelta,
		UploadMbps:       round1(ulMbps),
		UploadLossPct:    round1(ulLoss),
		UploadSkipped:    uploadSkipped,
		CompletedAt:      time.Now().Format(time.RFC3339),
	}

	r.mu.Lock()
	copyRes := result
	r.lastResult = &copyRes
	r.mu.Unlock()

	emit(BufferbloatProgress{
		Phase:         "done",
		ProgressPct:   100,
		CurrentPingMs: result.IdleMedianMs,
		CurrentMbps:   result.DownloadMbps,
		Message:       fmt.Sprintf("Test finalizado · Calificación %s", result.Grade),
	})

	return result, nil
}

func emitCancelledOrError(ctx context.Context, err error, emit func(BufferbloatProgress)) {
	if errors.Is(err, context.Canceled) || ctx.Err() != nil {
		emit(BufferbloatProgress{Phase: "cancelled", ProgressPct: 0, Message: "Test cancelado por el usuario."})
		return
	}
	emit(BufferbloatProgress{Phase: "error", ProgressPct: 0, Message: err.Error()})
}

func (r *BufferbloatRunner) measureLoadPhase(
	ctx context.Context,
	phase string,
	target string,
	dur time.Duration,
	pingInterval time.Duration,
	loadFn LoadFunc,
	pctStart int,
	pctEnd int,
	msg string,
	emit func(BufferbloatProgress),
) ([]float64, float64, float64, error) {
	phaseCtx, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	var totalBytes atomic.Int64
	loadErrCh := make(chan error, 1)

	go func() {
		loadErrCh <- loadFn(phaseCtx, func(n int64) {
			totalBytes.Add(n)
		})
	}()

	start := time.Now()
	samples, lossPct, err := r.measurePhase(ctx, phase, target, dur, pingInterval, &totalBytes, pctStart, pctEnd, msg, emit)
	elapsed := time.Since(start).Seconds()
	if err != nil {
		cancel()
		return nil, 0, 0, err
	}

	cancel()
	loadErr := <-loadErrCh

	mbps := 0.0
	if elapsed > 0 {
		mbps = (float64(totalBytes.Load()) * 8.0) / (elapsed * 1_000_000.0)
	}
	if loadErr != nil && totalBytes.Load() == 0 {
		return samples, lossPct, 0, loadErr
	}
	return samples, lossPct, mbps, nil
}

func (r *BufferbloatRunner) measurePhase(
	ctx context.Context,
	phase string,
	target string,
	dur time.Duration,
	pingInterval time.Duration,
	totalBytes *atomic.Int64,
	pctStart int,
	pctEnd int,
	msg string,
	emit func(BufferbloatProgress),
) ([]float64, float64, error) {
	samples := make([]float64, 0, int(dur/pingInterval)+2)
	sent := 0
	lost := 0

	start := time.Now()
	deadline := start.Add(dur)
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	probe := func() {
		probeCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		res := r.pinger.Ping(probeCtx, target, 800*time.Millisecond)
		cancel()

		sent++
		currentPing := 0.0
		if res.Success {
			samples = append(samples, res.RTTMs)
			currentPing = res.RTTMs
		} else {
			lost++
		}

		elapsed := time.Since(start)
		frac := float64(elapsed) / float64(dur)
		if frac > 1 {
			frac = 1
		}
		pct := pctStart + int(math.Round(frac*float64(pctEnd-pctStart)))

		mbps := 0.0
		if totalBytes != nil && elapsed.Seconds() > 0.1 {
			mbps = round1((float64(totalBytes.Load()) * 8.0) / (elapsed.Seconds() * 1_000_000.0))
		}

		emit(BufferbloatProgress{
			Phase:         phase,
			ProgressPct:   pct,
			CurrentPingMs: currentPing,
			CurrentMbps:   mbps,
			Message:       msg,
		})
	}

	probe()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				break
			}
			probe()
		}
	}

	lossPct := 0.0
	if sent > 0 {
		lossPct = float64(lost) * 100.0 / float64(sent)
	}
	return samples, lossPct, nil
}

func defaultHTTPDownloadLoad(ctx context.Context, onBytes func(int64)) error {
	client := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	var successCount atomic.Int32
	var firstErr atomic.Value

	for w := 0; w < defaultLoadConcurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			buf := make([]byte, 32*1024)
			urlIdx := 0

			for ctx.Err() == nil {
				url := defaultDownloadURLs[urlIdx%len(defaultDownloadURLs)]
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					firstErr.CompareAndSwap(nil, err)
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					firstErr.CompareAndSwap(nil, err)
					urlIdx++
					if urlIdx >= len(defaultDownloadURLs)*2 {
						return
					}
					continue
				}

				for ctx.Err() == nil {
					n, readErr := resp.Body.Read(buf)
					if n > 0 {
						successCount.Add(1)
						onBytes(int64(n))
					}
					if readErr != nil {
						break
					}
				}
				_ = resp.Body.Close()
			}
		}(w)
	}

	wg.Wait()
	if successCount.Load() == 0 {
		if err, ok := firstErr.Load().(error); ok && err != nil {
			return err
		}
		if ctx.Err() != nil && !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return ctx.Err()
		}
		return errors.New("no se pudo conectar al servidor de prueba de descarga")
	}
	return nil
}

type deterministicPayloadReader struct {
	remaining int64
	onBytes   func(int64)
}

func (r *deterministicPayloadReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > r.remaining {
		n = int(r.remaining)
	}
	for i := 0; i < n; i++ {
		p[i] = byte('N')
	}
	r.remaining -= int64(n)
	if r.onBytes != nil {
		r.onBytes(int64(n))
	}
	return n, nil
}

func defaultHTTPUploadLoad(ctx context.Context, onBytes func(int64)) error {
	client := &http.Client{Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	var uploadedAny atomic.Int32
	var firstErr atomic.Value

	for w := 0; w < defaultLoadConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				reader := &deterministicPayloadReader{
					remaining: defaultUploadChunkBytes,
					onBytes: func(n int64) {
						uploadedAny.Add(1)
						onBytes(n)
					},
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, defaultUploadURL, reader)
				if err != nil {
					firstErr.CompareAndSwap(nil, err)
					return
				}
				req.ContentLength = defaultUploadChunkBytes
				req.Header.Set("Content-Type", "application/octet-stream")

				resp, err := client.Do(req)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					firstErr.CompareAndSwap(nil, err)
					return
				}
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	if uploadedAny.Load() == 0 {
		if err, ok := firstErr.Load().(error); ok && err != nil {
			return err
		}
		return errors.New("no se pudo iniciar la carga de subida")
	}
	return nil
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return (cp[mid-1] + cp[mid]) / 2.0
}

func meanFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
