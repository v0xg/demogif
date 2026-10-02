package gifgen

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/nfnt/resize"
)

// maxFrameDelay caps how long a single frame is shown. Gaps between captures
// that are longer than this (AI calls and re-crawls at checkpoints, slow page
// loads) aren't part of the demo and would otherwise freeze the GIF.
const maxFrameDelay = time.Second

// Encoder builds a GIF incrementally: each frame is downscaled and quantized
// in the background as it is added, so only small paletted frames are kept in
// memory and capture timing isn't skewed by encoding work.
type Encoder struct {
	maxWidth uint
	fps      int
	frames   []*image.Paletted
	times    []time.Time

	mu      sync.Mutex
	wg      sync.WaitGroup
	workers chan struct{}
}

// NewEncoder creates an encoder that scales frames down to maxWidth (0 = 800).
// fps sets the display time of the last frame, which has no successor to measure against.
func NewEncoder(maxWidth uint, fps int) *Encoder {
	if maxWidth == 0 {
		maxWidth = 800
	}
	return &Encoder{maxWidth: maxWidth, fps: fps, workers: make(chan struct{}, runtime.NumCPU())}
}

// Add queues a frame captured at time at. It returns immediately; encoding
// runs on a bounded pool of workers and Write waits for it to finish.
func (e *Encoder) Add(frame image.Image, at time.Time) {
	e.mu.Lock()
	idx := len(e.frames)
	e.frames = append(e.frames, nil)
	e.times = append(e.times, at)
	e.mu.Unlock()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.workers <- struct{}{}
		defer func() { <-e.workers }()

		paletted := e.quantize(frame)

		e.mu.Lock()
		e.frames[idx] = paletted
		e.mu.Unlock()
	}()
}

// quantize downscales a frame and converts it to a paletted image
func (e *Encoder) quantize(frame image.Image) *image.Paletted {
	bounds := frame.Bounds()
	width := e.maxWidth
	if uint(bounds.Dx()) < width {
		width = uint(bounds.Dx())
	}
	height := uint(float64(width) * float64(bounds.Dy()) / float64(bounds.Dx()))

	resized := resize.Resize(width, height, frame, resize.Lanczos3)

	// Each frame gets its own palette so colors on later pages aren't
	// forced into the first page's palette
	paletted := image.NewPaletted(resized.Bounds(), generatePalette(resized))
	draw.FloydSteinberg.Draw(paletted, resized.Bounds(), resized, resized.Bounds().Min)
	return paletted
}

// Len returns the number of frames added so far
func (e *Encoder) Len() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.frames)
}

// Write encodes the GIF to outputPath and returns the file size.
// Frame delays come from the real capture times, so playback matches what happened.
func (e *Encoder) Write(outputPath string) (int64, error) {
	e.wg.Wait()
	if len(e.frames) == 0 {
		return 0, errors.New("no frames captured")
	}

	g := &gif.GIF{
		Image:     e.frames,
		Delay:     frameDelays(e.times, e.fps),
		LoopCount: 0, // Infinite loop
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if err := gif.EncodeAll(f, g); err != nil {
		return 0, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

// frameDelays converts capture times into GIF delays (1/100s units).
// Delays below 2 are raised to 2, since browsers treat 0-1 as "default" and play them slowly.
func frameDelays(times []time.Time, fps int) []int {
	delays := make([]int, len(times))
	for i := range times {
		gap := time.Second / time.Duration(fps)
		if i+1 < len(times) {
			gap = times[i+1].Sub(times[i])
		}
		if gap > maxFrameDelay {
			gap = maxFrameDelay
		}
		d := int((gap + 5*time.Millisecond) / (10 * time.Millisecond))
		if d < 2 {
			d = 2
		}
		delays[i] = d
	}
	return delays
}

// generatePalette creates a 256-color palette from the image's most frequent colors
func generatePalette(img image.Image) color.Palette {
	bounds := img.Bounds()
	colorMap := make(map[color.RGBA]int)

	// Sample colors from the image
	step := 4 // Sample every 4th pixel for performance
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, _ := img.At(x, y).RGBA()
			c := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
			colorMap[c]++
		}
	}

	type colorCount struct {
		c     color.RGBA
		count int
	}
	colors := make([]colorCount, 0, len(colorMap))
	for c, count := range colorMap {
		colors = append(colors, colorCount{c, count})
	}

	// Sort by count descending; break ties by value so identical frames get
	// identical palettes (map order is random, and differing palettes flicker)
	sort.Slice(colors, func(i, j int) bool {
		a, b := colors[i], colors[j]
		if a.count != b.count {
			return a.count > b.count
		}
		if a.c.R != b.c.R {
			return a.c.R < b.c.R
		}
		if a.c.G != b.c.G {
			return a.c.G < b.c.G
		}
		return a.c.B < b.c.B
	})

	palette := make(color.Palette, 0, 256)
	for i := 0; i < len(colors) && len(palette) < 256; i++ {
		palette = append(palette, colors[i].c)
	}

	// If we don't have enough colors, pad with grayscale
	for len(palette) < 256 {
		gray := uint8(len(palette))
		palette = append(palette, color.RGBA{gray, gray, gray, 255})
	}

	return palette
}
