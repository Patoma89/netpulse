package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo       = "Patoma89/netpulse"
	DefaultAPIBaseURL = "https://api.github.com"
)

// UpdateCheckResult summarizes a manual, user-initiated GitHub release check.
type UpdateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	ChecksumsURL    string `json:"checksumsUrl"`
	PublishedAt     string `json:"publishedAt"`
	Notes           string `json:"notes"`
	CheckedAt       string `json:"checkedAt"`
	Status          string `json:"status"` // "up_to_date" | "update_available" | "unavailable"
	Error           string `json:"error"`
}

type githubReleasePayload struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CompareSemver compares two semantic versions ("v1.2.0", "1.1.0") and returns -1, 0, or 1.
func CompareSemver(current, latest string) int {
	a := parseSemverParts(current)
	b := parseSemverParts(latest)
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func parseSemverParts(v string) [3]int {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "v"))
	if idx := strings.IndexAny(trimmed, "-+"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	parts := strings.Split(trimmed, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil && n >= 0 {
			out[i] = n
		}
	}
	return out
}

// CheckLatestRelease performs a single opt-in request to GitHub Releases API when invoked by the user.
func CheckLatestRelease(ctx context.Context, currentVersion, repo, apiBaseURL string) UpdateCheckResult {
	res := UpdateCheckResult{
		CurrentVersion: strings.TrimPrefix(strings.TrimSpace(currentVersion), "v"),
		CheckedAt:      time.Now().Format("15:04:05"),
		Status:         "unavailable",
	}

	targetRepo := strings.TrimSpace(repo)
	if targetRepo == "" {
		targetRepo = DefaultRepo
	}
	base := strings.TrimRight(strings.TrimSpace(apiBaseURL), "/")
	if base == "" {
		base = DefaultAPIBaseURL
	}

	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", base, targetRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "NetPulse/"+res.CurrentVersion)

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		res.Error = fmt.Sprintf("sin acceso al repositorio (%v)", err)
		return res
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		res.Error = fmt.Sprintf("GitHub devolvió HTTP %d para %s", resp.StatusCode, targetRepo)
		return res
	}

	var payload githubReleasePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		res.Error = fmt.Sprintf("respuesta inválida de GitHub: %v", err)
		return res
	}

	latest := strings.TrimPrefix(strings.TrimSpace(payload.TagName), "v")
	if latest == "" {
		res.Error = "el release no contiene tag_name"
		return res
	}

	res.LatestVersion = latest
	res.ReleaseURL = payload.HTMLURL
	res.PublishedAt = payload.PublishedAt
	res.Notes = truncateNotes(payload.Body, 280)
	for _, asset := range payload.Assets {
		if strings.EqualFold(asset.Name, "SHA256SUMS.txt") {
			res.ChecksumsURL = asset.BrowserDownloadURL
			break
		}
	}

	if CompareSemver(res.CurrentVersion, res.LatestVersion) < 0 {
		res.UpdateAvailable = true
		res.Status = "update_available"
	} else {
		res.Status = "up_to_date"
	}
	return res
}

func truncateNotes(s string, max int) string {
	cleaned := strings.TrimSpace(s)
	if len(cleaned) <= max {
		return cleaned
	}
	return strings.TrimSpace(cleaned[:max]) + "..."
}
