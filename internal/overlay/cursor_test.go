package overlay

import (
	"image"
	"image/color"
	"testing"

	"github.com/v0xg/demogif/internal/executor"
)

func whiteFrame(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	return img
}

func TestDrawCursorLeavesInputUntouched(t *testing.T) {
	frame := whiteFrame(100, 100)
	out := DrawCursor(frame, executor.CursorPosition{X: 50, Y: 50})

	if frame.RGBAAt(50, 50) != (color.RGBA{255, 255, 255, 255}) {
		t.Fatal("DrawCursor modified the input frame")
	}
	// The cursor tip is outlined in black
	if r, g, b, _ := out.At(50, 50).RGBA(); r != 0 || g != 0 || b != 0 {
		t.Fatalf("expected cursor outline at tip, got %v", out.At(50, 50))
	}
}

func TestDrawCursorAtOriginIsSkipped(t *testing.T) {
	frame := whiteFrame(20, 20)
	out := DrawCursor(frame, executor.CursorPosition{})
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if r, _, _, _ := out.At(x, y).RGBA(); r != 0xffff {
				t.Fatalf("pixel (%d,%d) changed for an unpositioned cursor", x, y)
			}
		}
	}
}

func TestDrawCursorNearEdgeDoesNotPanic(t *testing.T) {
	frame := whiteFrame(30, 30)
	for _, pos := range []executor.CursorPosition{
		{X: 29, Y: 29, Click: true},
		{X: -5, Y: 10, Click: true},
		{X: 10, Y: 500},
	} {
		DrawCursor(frame, pos)
	}
}
