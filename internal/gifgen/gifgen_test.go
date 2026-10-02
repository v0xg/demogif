package gifgen

import (
	"image"
	"image/color"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestFrameDelays(t *testing.T) {
	start := time.Unix(0, 0)
	times := []time.Time{
		start,
		start.Add(120 * time.Millisecond), // 12
		start.Add(125 * time.Millisecond), // 5ms gap -> clamped to 2
		start.Add(5 * time.Second),        // long gap -> capped at 1s
	}
	got := frameDelays(times, 20)
	want := []int{12, 2, 100, 5} // last frame uses 1/fps
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frameDelays = %v, want %v", got, want)
	}
}

func TestGeneratePaletteDeterministic(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	r := rand.New(rand.NewSource(1))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.Intn(256))
	}
	// Many colors share the same count, so map order would leak into the palette without a tiebreak
	first := generatePalette(img)
	for i := 0; i < 5; i++ {
		if !reflect.DeepEqual(first, generatePalette(img)) {
			t.Fatal("palette differs between runs for the same image")
		}
	}
	if len(first) != 256 {
		t.Fatalf("palette has %d colors, want 256", len(first))
	}
}

func TestEncoderPreservesOrder(t *testing.T) {
	e := NewEncoder(800, 20)
	start := time.Unix(0, 0)
	for i := 0; i < 20; i++ {
		img := image.NewRGBA(image.Rect(0, 0, 64, 48))
		c := color.RGBA{uint8(i * 10), 0, 0, 255}
		for p := 0; p < len(img.Pix); p += 4 {
			img.Pix[p], img.Pix[p+1], img.Pix[p+2], img.Pix[p+3] = c.R, c.G, c.B, c.A
		}
		e.Add(img, start.Add(time.Duration(i)*100*time.Millisecond))
	}
	e.wg.Wait()
	for i, f := range e.frames {
		r, _, _, _ := f.At(0, 0).RGBA()
		if want := uint32(i * 10); r>>8 != want {
			t.Fatalf("frame %d has red %d, want %d", i, r>>8, want)
		}
	}
}
