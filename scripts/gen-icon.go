//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	img512 := renderIcon(512)

	if err := savePNG(filepath.Join("build", "appicon.png"), img512); err != nil {
		panic(err)
	}
	if err := savePNG(filepath.Join("frontend", "src", "assets", "images", "logo-universal.png"), img512); err != nil {
		panic(err)
	}

	sizes := []int{256, 64, 48, 32, 16}
	if err := saveICO(filepath.Join("build", "windows", "icon.ico"), sizes); err != nil {
		panic(err)
	}
}

func renderIcon(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	s := float64(size)
	cx, cy := s/2.0, s/2.0
	margin := s * 0.055
	halfBox := s/2.0 - margin
	cornerR := s * 0.22

	// Telemetry pulse waveform normalized in [-1..1] x space
	pulseY := func(nx float64) float64 {
		// Flat outside [-0.62, 0.62], sharp ECG-like pulse in middle
		if nx < -0.62 || nx > 0.62 {
			return 0
		}
		if nx >= -0.62 && nx < -0.22 {
			return 0
		}
		if nx >= -0.22 && nx < -0.10 {
			return -0.22 * math.Sin(((nx+0.22)/0.12)*math.Pi)
		}
		if nx >= -0.10 && nx < 0.02 {
			return 0.44 * math.Sin(((nx+0.10)/0.12)*math.Pi)
		}
		if nx >= 0.02 && nx < 0.14 {
			return -0.28 * math.Sin(((nx-0.02)/0.12)*math.Pi)
		}
		return 0
	}

	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			x := float64(px) + 0.5
			y := float64(py) + 0.5

			// Signed distance to rounded squircle
			dx := math.Abs(x-cx) - (halfBox - cornerR)
			dy := math.Abs(y-cy) - (halfBox - cornerR)
			qx := math.Max(dx, 0)
			qy := math.Max(dy, 0)
			sdf := math.Hypot(qx, qy) + math.Min(math.Max(dx, dy), 0) - cornerR

			if sdf > 1.0 {
				continue
			}

			alpha := uint8(255)
			if sdf > -1.0 {
				a := 1.0 - (sdf+1.0)/2.0
				if a < 0 {
					a = 0
				}
				if a > 1 {
					a = 1
				}
				alpha = uint8(a * 255)
			}

			// Base dark background with subtle radial cyan glow
			dist := math.Hypot(x-cx, y-cy) / (s * 0.5)
			r := 9.0 + (1.0-math.Min(dist, 1.0))*10.0
			g := 13.0 + (1.0-math.Min(dist, 1.0))*22.0
			b := 22.0 + (1.0-math.Min(dist, 1.0))*36.0

			// Border highlight
			if sdf > -s*0.022 {
				r = mix(r, 6, 0.25)
				g = mix(g, 182, 0.25)
				b = mix(b, 212, 0.25)
			}

			// Concentric radar rings (Cyan #06B6D4)
			rad := math.Hypot(x-cx, y-cy)
			ringW := math.Max(1.6, s*0.020)
			for _, rf := range []float64{0.34, 0.22} {
				ringR := s * rf
				dRing := math.Abs(rad - ringR)
				if dRing < ringW*1.6 {
					t := math.Exp(-(dRing * dRing) / (ringW * ringW * 0.5))
					r = mix(r, 6, t*0.92)
					g = mix(g, 182, t*0.92)
					b = mix(b, 212, t*0.92)
				}
			}

			// Subtle radar sweep wedge in upper-right quadrant
			ang := math.Atan2(y-cy, x-cx)
			if rad < s*0.34 && ang > -math.Pi*0.42 && ang < -math.Pi*0.05 {
				wedge := 1.0 - (-math.Pi*0.05-ang)/(math.Pi*0.37)
				r = mix(r, 6, wedge*0.18)
				g = mix(g, 182, wedge*0.18)
				b = mix(b, 212, wedge*0.18)
			}

			// Telemetry pulse waveform (Emerald #10B981)
			nx := (x - cx) / (s * 0.42)
			ny := (y - cy) / (s * 0.42)
			if math.Abs(nx) <= 0.68 {
				pyNorm := -pulseY(nx)
				dWave := math.Abs(ny-pyNorm) * (s * 0.42)
				strokeW := math.Max(1.8, s*0.024)
				if dWave < strokeW*1.6 {
					t := math.Exp(-(dWave * dWave) / (strokeW * strokeW * 0.5))
					r = mix(r, 16, t)
					g = mix(g, 185, t)
					b = mix(b, 129, t)
				}
			}

			// Glowing center beacon
			beaconR := s * 0.055
			if rad < beaconR*2.2 {
				t := math.Exp(-(rad * rad) / (beaconR * beaconR * 0.7))
				r = mix(r, 34, t)
				g = mix(g, 211, t)
				b = mix(b, 238, t)
			}
			if rad < s*0.032 {
				r, g, b = 224, 247, 250
			}

			img.SetNRGBA(px, py, color.NRGBA{
				R: clampU8(r),
				G: clampU8(g),
				B: clampU8(b),
				A: alpha,
			})
		}
	}
	return img
}

func mix(a, b, t float64) float64 {
	return a + (b-a)*t
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func saveICO(path string, sizes []int) error {
	type icoFrame struct {
		size int
		png  []byte
	}
	frames := make([]icoFrame, 0, len(sizes))
	for _, sz := range sizes {
		var buf bytes.Buffer
		if err := png.Encode(&buf, renderIcon(sz)); err != nil {
			return err
		}
		frames = append(frames, icoFrame{size: sz, png: buf.Bytes()})
	}

	var out bytes.Buffer
	// ICONDIR header (6 bytes)
	_ = binary.Write(&out, binary.LittleEndian, uint16(0))
	_ = binary.Write(&out, binary.LittleEndian, uint16(1))
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(frames)))

	offset := uint32(6 + 16*len(frames))
	for _, fr := range frames {
		dim := byte(fr.size)
		if fr.size >= 256 {
			dim = 0
		}
		out.WriteByte(dim)
		out.WriteByte(dim)
		out.WriteByte(0)
		out.WriteByte(0)
		_ = binary.Write(&out, binary.LittleEndian, uint16(1))
		_ = binary.Write(&out, binary.LittleEndian, uint16(32))
		_ = binary.Write(&out, binary.LittleEndian, uint32(len(fr.png)))
		_ = binary.Write(&out, binary.LittleEndian, offset)
		offset += uint32(len(fr.png))
	}
	for _, fr := range frames {
		out.Write(fr.png)
	}

	return os.WriteFile(path, out.Bytes(), 0o644)
}
