package gifgen

import (
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"os"

	"github.com/nfnt/resize"
)

// Options configures GIF generation
type Options struct {
	FPS      int
	MaxWidth uint
}

// Generate creates a GIF from frames
func Generate(frames []image.Image, outputPath string, opts Options) (int64, error) {
	if len(frames) == 0 {
		return 0, nil
	}

	// Calculate delay (in 100ths of a second)
	delay := 100 / opts.FPS

	// Determine output size
	bounds := frames[0].Bounds()
	outputWidth := opts.MaxWidth
	if outputWidth == 0 {
		outputWidth = 800
	}

	// Calculate height maintaining aspect ratio
	aspectRatio := float64(bounds.Dy()) / float64(bounds.Dx())
	outputHeight := uint(float64(outputWidth) * aspectRatio)

	// Create GIF
	g := &gif.GIF{
		Image:     make([]*image.Paletted, len(frames)),
		Delay:     make([]int, len(frames)),
		LoopCount: 0, // Infinite loop
	}

	// Generate optimized palette from first frame
	palette := generatePalette(frames[0])

	for i, frame := range frames {
		// Resize frame
		resized := resize.Resize(outputWidth, outputHeight, frame, resize.Lanczos3)

		// Convert to paletted image
		paletted := image.NewPaletted(resized.Bounds(), palette)
		draw.FloydSteinberg.Draw(paletted, resized.Bounds(), resized, image.Point{})

		g.Image[i] = paletted
		g.Delay[i] = delay
	}

	// Write to file
	f, err := os.Create(outputPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if err := gif.EncodeAll(f, g); err != nil {
		return 0, err
	}

	// Get file size
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	return info.Size(), nil
}

// generatePalette creates an optimized 256-color palette from the image
func generatePalette(img image.Image) color.Palette {
	// Use a simple median cut algorithm approximation
	// For better quality, could use more sophisticated quantization

	bounds := img.Bounds()
	colorMap := make(map[color.RGBA]int)

	// Sample colors from the image
	step := 4 // Sample every 4th pixel for performance
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			r, g, b, a := img.At(x, y).RGBA()
			c := color.RGBA{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
				A: uint8(a >> 8),
			}
			colorMap[c]++
		}
	}

	// Sort colors by frequency and take top 255
	type colorCount struct {
		c     color.RGBA
		count int
	}
	colors := make([]colorCount, 0, len(colorMap))
	for c, count := range colorMap {
		colors = append(colors, colorCount{c, count})
	}

	// Sort by count descending
	for i := 0; i < len(colors)-1; i++ {
		for j := i + 1; j < len(colors); j++ {
			if colors[j].count > colors[i].count {
				colors[i], colors[j] = colors[j], colors[i]
			}
		}
	}

	// Create palette with most common colors
	palette := make(color.Palette, 0, 256)

	// Add transparent color first
	palette = append(palette, color.RGBA{0, 0, 0, 0})

	// Add most frequent colors
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
