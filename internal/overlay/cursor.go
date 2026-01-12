package overlay

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/v0xg/demogif/internal/executor"
)

// CursorSize is the size of the cursor sprite
const CursorSize = 20

// ApplyCursor draws cursor and click effects on frames
func ApplyCursor(frames []image.Image, positions []executor.CursorPosition) ([]image.Image, error) {
	if len(positions) == 0 {
		return frames, nil
	}

	result := make([]image.Image, len(frames))

	// Interpolate cursor positions between frames
	interpolated := interpolatePositions(positions, len(frames))

	for i, frame := range frames {
		pos := interpolated[i]
		result[i] = drawCursorOnFrame(frame, pos)
	}

	return result, nil
}

// interpolatePositions creates smooth cursor movement between known positions
func interpolatePositions(positions []executor.CursorPosition, frameCount int) []executor.CursorPosition {
	if len(positions) == 0 {
		return make([]executor.CursorPosition, frameCount)
	}

	result := make([]executor.CursorPosition, frameCount)

	// Simple approach: map positions to frames
	for i := 0; i < frameCount; i++ {
		// Find which position this frame corresponds to
		posIdx := int(float64(i) / float64(frameCount) * float64(len(positions)))
		if posIdx >= len(positions) {
			posIdx = len(positions) - 1
		}

		currentPos := positions[posIdx]

		// If not the last position, interpolate towards next
		if posIdx < len(positions)-1 {
			nextPos := positions[posIdx+1]
			progress := (float64(i)/float64(frameCount)*float64(len(positions)) - float64(posIdx))

			// Ease-in-out interpolation
			progress = easeInOut(progress)

			result[i] = executor.CursorPosition{
				X:     int(float64(currentPos.X) + progress*(float64(nextPos.X)-float64(currentPos.X))),
				Y:     int(float64(currentPos.Y) + progress*(float64(nextPos.Y)-float64(currentPos.Y))),
				State: currentPos.State,
				Click: currentPos.Click,
			}
		} else {
			result[i] = currentPos
		}
	}

	return result
}

// easeInOut provides smooth acceleration and deceleration
func easeInOut(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return 1 - math.Pow(-2*t+2, 2)/2
}

// drawCursorOnFrame creates a new image with cursor overlay
func drawCursorOnFrame(frame image.Image, pos executor.CursorPosition) image.Image {
	bounds := frame.Bounds()
	result := image.NewRGBA(bounds)

	// Copy original frame
	draw.Draw(result, bounds, frame, bounds.Min, draw.Src)

	// Skip if cursor is at origin (not yet positioned)
	if pos.X == 0 && pos.Y == 0 {
		return result
	}

	// Draw click ripple effect if clicking
	if pos.Click {
		drawClickRipple(result, pos.X, pos.Y)
	}

	// Draw cursor
	drawCursor(result, pos.X, pos.Y, pos.State)

	return result
}

// drawCursor draws a simple arrow cursor
func drawCursor(img *image.RGBA, x, y int, state executor.CursorState) {
	// Cursor outline (black)
	cursorColor := color.RGBA{0, 0, 0, 255}
	// Cursor fill (white)
	fillColor := color.RGBA{255, 255, 255, 255}

	// Simple arrow cursor shape
	// Points define the cursor outline
	cursorPoints := []struct{ dx, dy int }{
		{0, 0},
		{0, 16},
		{4, 12},
		{7, 18},
		{10, 17},
		{7, 11},
		{12, 11},
	}

	// Draw cursor fill
	for dy := 0; dy < 18; dy++ {
		for dx := 0; dx < 13; dx++ {
			if isInsideCursor(dx, dy) {
				setPixelSafe(img, x+dx, y+dy, fillColor)
			}
		}
	}

	// Draw cursor outline
	for i := 0; i < len(cursorPoints); i++ {
		p1 := cursorPoints[i]
		p2 := cursorPoints[(i+1)%len(cursorPoints)]
		drawLine(img, x+p1.dx, y+p1.dy, x+p2.dx, y+p2.dy, cursorColor)
	}
}

// isInsideCursor checks if a point is inside the cursor shape
func isInsideCursor(dx, dy int) bool {
	// Simple triangular cursor approximation
	if dy < 0 || dy > 16 {
		return false
	}
	if dx < 0 {
		return false
	}

	// Main triangle part
	if dy <= 11 {
		return dx <= dy*12/16 && dx >= 0
	}

	// Arrow shaft part
	if dy <= 16 && dx >= 0 && dx <= 4 {
		return true
	}

	return false
}

// drawLine draws a line between two points using Bresenham's algorithm
func drawLine(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}
	err := dx - dy

	for {
		setPixelSafe(img, x1, y1, c)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

// drawClickRipple draws an expanding circle ripple effect
func drawClickRipple(img *image.RGBA, x, y int) {
	rippleColor := color.RGBA{66, 133, 244, 100} // Semi-transparent blue
	radius := 15

	// Draw circle outline
	for angle := 0.0; angle < 360; angle += 1 {
		rad := angle * math.Pi / 180
		px := x + int(float64(radius)*math.Cos(rad))
		py := y + int(float64(radius)*math.Sin(rad))
		setPixelSafe(img, px, py, rippleColor)
		setPixelSafe(img, px+1, py, rippleColor)
		setPixelSafe(img, px, py+1, rippleColor)
	}
}

func setPixelSafe(img *image.RGBA, x, y int, c color.RGBA) {
	bounds := img.Bounds()
	if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
		img.Set(x, y, c)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
