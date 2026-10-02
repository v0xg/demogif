package executor

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/v0xg/demogif/internal/crawler"
)

// Options configures execution behavior
type Options struct {
	FPS       int
	BaseDelay int // Base delay between actions in ms
	Verbose   bool
	// OnFrame receives every captured frame as soon as its action finishes,
	// so callers can encode incrementally instead of holding the whole recording
	OnFrame func(FrameData)
}

// FrameData holds a captured frame with its cursor state and capture time
type FrameData struct {
	Image  image.Image
	Cursor CursorPosition
	At     time.Time
}

// ExecuteResult holds the result of executing a batch of actions
type ExecuteResult struct {
	LastCursor CursorPosition
	// StoppedAt is the index of the action that ended the batch early
	// (a checkpoint or a failure), or -1 if every action ran
	StoppedAt int
	// Err is the failure that stopped the batch, nil for a checkpoint or full run
	Err error
}

// ExecuteBatch runs actions until a checkpoint is hit, an action fails, or all actions complete.
// A failure stops the batch so the caller can re-crawl and let the AI re-plan around it.
func ExecuteBatch(browser *crawler.Browser, actions []Action, opts Options, startCursor *CursorPosition) *ExecuteResult {
	page := browser.Page()

	// Frame timing based on FPS
	frameInterval := time.Duration(1000/opts.FPS) * time.Millisecond

	// Current cursor position
	currentCursor := CursorPosition{X: 640, Y: 360, State: CursorDefault}
	if startCursor != nil {
		currentCursor = *startCursor
	}

	result := &ExecuteResult{StoppedAt: -1}
	emit := func(frames []FrameData) {
		if opts.OnFrame == nil {
			return
		}
		for _, f := range frames {
			opts.OnFrame(f)
		}
	}

	for i, action := range actions {
		if opts.Verbose {
			fmt.Printf("  [%d/%d] %s %s", i+1, len(actions), action.Type, action.Selector)
		}

		// Execute the action with animation
		newFrames, newCursor, err := executeActionAnimated(page, action, currentCursor, opts, frameInterval)
		if err != nil {
			if opts.Verbose {
				fmt.Printf(" ✗ (%v)\n", err)
			}
			result.StoppedAt = i
			result.Err = err
			break
		}

		if opts.Verbose {
			if action.Checkpoint {
				fmt.Println(" ✓ [checkpoint]")
			} else {
				fmt.Println(" ✓")
			}
		}

		emit(newFrames)
		currentCursor = newCursor

		// Post-action wait with frame capture ("wait" actions already waited)
		if action.Type != "wait" {
			waitTime := action.Duration
			if waitTime == 0 {
				waitTime = opts.BaseDelay
			}
			emit(captureWaitFrames(page, currentCursor, waitTime, frameInterval))
		}

		// If this was a checkpoint, stop and signal re-crawl needed
		if action.Checkpoint {
			result.StoppedAt = i
			break
		}
	}

	result.LastCursor = currentCursor
	return result
}

// elementTimeout bounds how long an action waits for its selector to appear.
// Without it, rod retries forever and a bad AI-generated selector hangs the run.
const elementTimeout = 5 * time.Second

// findElement looks up a selector with a bounded wait
func findElement(page *rod.Page, selector string) (*rod.Element, error) {
	el, err := page.Timeout(elementTimeout).Element(selector)
	if err != nil {
		return nil, fmt.Errorf("element not found: %s", selector)
	}
	return el.CancelTimeout(), nil
}

// executeActionAnimated executes an action with animated frames
func executeActionAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	switch action.Type {
	case "click":
		return executeClickAnimated(page, action, currentCursor, opts, frameInterval)
	case "type":
		return executeTypeAnimated(page, action, currentCursor, opts, frameInterval)
	case "scroll":
		return executeScrollAnimated(page, action, currentCursor, opts, frameInterval)
	case "hover":
		return executeHoverAnimated(page, action, currentCursor, opts, frameInterval)
	case "wait":
		waitTime := action.Duration
		if waitTime == 0 {
			waitTime = opts.BaseDelay
		}
		return captureWaitFrames(page, currentCursor, waitTime, frameInterval), currentCursor, nil
	case "navigate":
		if err := page.Navigate(action.URL); err != nil {
			return nil, currentCursor, fmt.Errorf("navigate failed: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return nil, currentCursor, fmt.Errorf("page load failed: %w", err)
		}
		var frames []FrameData
		if frame, err := captureFrame(page); err == nil {
			frames = append(frames, FrameData{Image: frame, At: time.Now(), Cursor: currentCursor})
		}
		return frames, currentCursor, nil
	default:
		return nil, currentCursor, fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// animateCursorMove moves the mouse from current to (x, y) with easing, capturing a frame per step
func animateCursorMove(page *rod.Page, currentCursor CursorPosition, x, y int, state CursorState, opts Options, frameInterval time.Duration) ([]FrameData, error) {
	movementFrames := opts.FPS / 2
	if movementFrames < 5 {
		movementFrames = 5
	}

	var frames []FrameData
	for i := 0; i <= movementFrames; i++ {
		t := float64(i) / float64(movementFrames)
		t = easeInOutQuad(t) // Smooth easing

		interpX := int(float64(currentCursor.X) + t*(float64(x)-float64(currentCursor.X)))
		interpY := int(float64(currentCursor.Y) + t*(float64(y)-float64(currentCursor.Y)))

		// Move actual mouse
		if err := page.Mouse.MoveTo(proto.Point{X: float64(interpX), Y: float64(interpY)}); err != nil {
			return nil, fmt.Errorf("mouse move failed: %w", err)
		}

		frame, err := captureFrame(page)
		if err != nil {
			continue
		}

		cursor := CursorPosition{X: interpX, Y: interpY, State: state}
		frames = append(frames, FrameData{Image: frame, At: time.Now(), Cursor: cursor})

		time.Sleep(frameInterval / 2) // Faster for movement
	}
	return frames, nil
}

// executeClickAnimated performs a click with cursor movement animation
func executeClickAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	el, err := findElement(page, action.Selector)
	if err != nil {
		return nil, currentCursor, err
	}

	x, y, err := getElementCenter(el)
	if err != nil {
		return nil, currentCursor, err
	}

	// Animate cursor movement to target (over ~0.5 seconds)
	frames, err := animateCursorMove(page, currentCursor, x, y, CursorPointer, opts, frameInterval)
	if err != nil {
		return nil, currentCursor, err
	}

	// Perform actual click
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, currentCursor, fmt.Errorf("click failed: %w", err)
	}

	// Capture click frames (show click indicator for ~0.3 seconds)
	clickFrames := opts.FPS / 3
	if clickFrames < 3 {
		clickFrames = 3
	}
	for i := 0; i < clickFrames; i++ {
		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		cursor := CursorPosition{X: x, Y: y, State: CursorPointer, Click: true}
		frames = append(frames, FrameData{Image: frame, At: time.Now(), Cursor: cursor})
		time.Sleep(frameInterval)
	}

	return frames, CursorPosition{X: x, Y: y, State: CursorPointer}, nil
}

// executeTypeAnimated performs typing with character-by-character animation
func executeTypeAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	el, err := findElement(page, action.Selector)
	if err != nil {
		return nil, currentCursor, err
	}

	x, y, err := getElementCenter(el)
	if err != nil {
		return nil, currentCursor, err
	}

	// Animate cursor movement to input field
	frames, err := animateCursorMove(page, currentCursor, x, y, CursorText, opts, frameInterval)
	if err != nil {
		return nil, currentCursor, err
	}

	// Click to focus
	if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return nil, currentCursor, fmt.Errorf("focus failed: %w", err)
	}

	// Clear existing text
	if err := el.SelectAllText(); err != nil {
		return nil, currentCursor, fmt.Errorf("select text failed: %w", err)
	}

	// Capture frame after focus
	if frame, err := captureFrame(page); err == nil {
		frames = append(frames, FrameData{
			Image:  frame,
			At:     time.Now(),
			Cursor: CursorPosition{X: x, Y: y, State: CursorText},
		})
	}

	// Type character by character
	runes := []rune(action.Text)
	typingDelay := 50 * time.Millisecond // 50ms between characters
	frameEvery := 2                      // Capture frame every N characters

	for i, char := range runes {
		// Type the character
		if err := typeRune(page, char); err != nil {
			return nil, currentCursor, fmt.Errorf("typing failed: %w", err)
		}

		// Capture frame every few characters
		if i%frameEvery == 0 || i == len(runes)-1 {
			time.Sleep(typingDelay)
			frame, err := captureFrame(page)
			if err != nil {
				continue
			}
			frames = append(frames, FrameData{
				Image:  frame,
				At:     time.Now(),
				Cursor: CursorPosition{X: x, Y: y, State: CursorText},
			})
		} else {
			time.Sleep(typingDelay / 2)
		}
	}

	// Hold on completed text for a moment
	for i := 0; i < opts.FPS/4; i++ {
		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		frames = append(frames, FrameData{
			Image:  frame,
			At:     time.Now(),
			Cursor: CursorPosition{X: x, Y: y, State: CursorText},
		})
		time.Sleep(frameInterval)
	}

	return frames, CursorPosition{X: x, Y: y, State: CursorText}, nil
}

// typeRune sends printable ASCII as key events and inserts anything else
// (accents, emoji, newlines) as text, since rod panics on keys outside its US keymap
func typeRune(page *rod.Page, r rune) error {
	if r >= 0x20 && r < 0x7f {
		return page.Keyboard.Type(input.Key(r))
	}
	return page.InsertText(string(r))
}

// executeScrollAnimated performs scroll with animation
func executeScrollAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	var frames []FrameData

	scrollSteps := 10
	stepX := float64(action.X) / float64(scrollSteps)
	stepY := float64(action.Y) / float64(scrollSteps)

	for i := 0; i < scrollSteps; i++ {
		if err := page.Mouse.Scroll(stepX, stepY, 1); err != nil {
			return nil, currentCursor, fmt.Errorf("scroll failed: %w", err)
		}
		time.Sleep(frameInterval)

		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		frames = append(frames, FrameData{Image: frame, At: time.Now(), Cursor: currentCursor})
	}

	return frames, currentCursor, nil
}

// executeHoverAnimated performs hover with cursor movement animation
func executeHoverAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	el, err := findElement(page, action.Selector)
	if err != nil {
		return nil, currentCursor, err
	}

	x, y, err := getElementCenter(el)
	if err != nil {
		return nil, currentCursor, err
	}

	// Animate cursor movement
	frames, err := animateCursorMove(page, currentCursor, x, y, CursorPointer, opts, frameInterval)
	if err != nil {
		return nil, currentCursor, err
	}

	// Trigger hover
	if err := el.Hover(); err != nil {
		return nil, currentCursor, fmt.Errorf("hover failed: %w", err)
	}

	// Capture hover state
	for i := 0; i < opts.FPS/4; i++ {
		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		frames = append(frames, FrameData{
			Image:  frame,
			At:     time.Now(),
			Cursor: CursorPosition{X: x, Y: y, State: CursorPointer},
		})
		time.Sleep(frameInterval)
	}

	return frames, CursorPosition{X: x, Y: y, State: CursorPointer}, nil
}

// captureWaitFrames captures frames for waitMs of wall-clock time (at least one frame).
// Bounded by time rather than frame count, since each screenshot itself takes time.
func captureWaitFrames(page *rod.Page, cursor CursorPosition, waitMs int, frameInterval time.Duration) []FrameData {
	var frames []FrameData

	deadline := time.Now().Add(time.Duration(waitMs) * time.Millisecond)
	for {
		next := time.Now().Add(frameInterval)
		if frame, err := captureFrame(page); err == nil {
			frames = append(frames, FrameData{Image: frame, At: time.Now(), Cursor: cursor})
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(time.Until(next))
	}

	return frames
}

// easeInOutQuad provides smooth acceleration/deceleration
func easeInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	}
	return 1 - (-2*t+2)*(-2*t+2)/2
}

func getElementCenter(el *rod.Element) (int, int, error) {
	box, err := el.Shape()
	if err != nil {
		return 0, 0, err
	}

	if len(box.Quads) == 0 {
		return 0, 0, fmt.Errorf("element has no shape")
	}

	quad := box.Quads[0]
	x := int((quad[0] + quad[2] + quad[4] + quad[6]) / 4)
	y := int((quad[1] + quad[3] + quad[5] + quad[7]) / 4)

	return x, y, nil
}

func captureFrame(page *rod.Page) (image.Image, error) {
	quality := 90
	data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatPng,
		Quality: &quality,
	})
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return img, nil
}
