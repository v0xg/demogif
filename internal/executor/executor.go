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
}

// FrameData holds a captured frame with its cursor state
type FrameData struct {
	Image  image.Image
	Cursor CursorPosition
}

// ExecuteResult holds the result of executing a batch of actions
type ExecuteResult struct {
	Frames          []image.Image
	CursorPositions []CursorPosition
	LastCursor      CursorPosition
	HitCheckpoint   bool
	CheckpointIndex int // Index of the checkpoint action that was hit (-1 if none)
}

// ExecuteBatch runs actions until a checkpoint is hit or all actions complete
// Returns frames, positions, and whether a checkpoint was encountered
func ExecuteBatch(browser *crawler.Browser, actions []Action, opts Options, startCursor *CursorPosition) (*ExecuteResult, error) {
	page := browser.Page()
	var frameData []FrameData

	// Frame timing based on FPS
	frameInterval := time.Duration(1000/opts.FPS) * time.Millisecond

	// Current cursor position
	currentCursor := CursorPosition{X: 640, Y: 360, State: CursorDefault}
	if startCursor != nil {
		currentCursor = *startCursor
	}

	result := &ExecuteResult{
		CheckpointIndex: -1,
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
			continue
		}

		if opts.Verbose {
			if action.Checkpoint {
				fmt.Println(" ✓ [checkpoint]")
			} else {
				fmt.Println(" ✓")
			}
		}

		frameData = append(frameData, newFrames...)
		currentCursor = newCursor

		// Post-action wait with frame capture
		waitTime := action.Duration
		if waitTime == 0 {
			waitTime = opts.BaseDelay
		}
		waitFrames := captureWaitFrames(page, currentCursor, waitTime, frameInterval)
		frameData = append(frameData, waitFrames...)

		// If this was a checkpoint, stop and signal re-crawl needed
		if action.Checkpoint {
			result.HitCheckpoint = true
			result.CheckpointIndex = i
			break
		}
	}

	// Extract images and positions
	result.Frames = make([]image.Image, len(frameData))
	result.CursorPositions = make([]CursorPosition, len(frameData))
	for i, fd := range frameData {
		result.Frames[i] = fd.Image
		result.CursorPositions[i] = fd.Cursor
	}
	result.LastCursor = currentCursor

	return result, nil
}

// Execute runs the action sequence and captures frames with animation
// Deprecated: Use ExecuteBatch for checkpoint support
func Execute(browser *crawler.Browser, actions []Action, opts Options) ([]image.Image, []CursorPosition, error) {
	page := browser.Page()
	var frameData []FrameData

	// Frame timing based on FPS
	frameInterval := time.Duration(1000/opts.FPS) * time.Millisecond

	// Current cursor position (starts at center of screen)
	currentCursor := CursorPosition{X: 640, Y: 360, State: CursorDefault}

	// Capture initial frames (hold for ~1 second)
	initialFrames := opts.FPS // 1 second worth of frames
	for i := 0; i < initialFrames; i++ {
		frame, err := captureFrame(page)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to capture initial frame: %w", err)
		}
		frameData = append(frameData, FrameData{Image: frame, Cursor: currentCursor})
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
			continue
		}

		if opts.Verbose {
			fmt.Println(" ✓")
		}

		frameData = append(frameData, newFrames...)
		currentCursor = newCursor

		// Post-action wait with frame capture
		waitTime := action.Duration
		if waitTime == 0 {
			waitTime = opts.BaseDelay
		}
		waitFrames := captureWaitFrames(page, currentCursor, waitTime, frameInterval)
		frameData = append(frameData, waitFrames...)
	}

	// Final hold frames (~1 second)
	finalFrames := opts.FPS
	for i := 0; i < finalFrames; i++ {
		frame, err := captureFrame(page)
		if err == nil {
			frameData = append(frameData, FrameData{Image: frame, Cursor: currentCursor})
		}
	}

	// Extract images and positions
	images := make([]image.Image, len(frameData))
	positions := make([]CursorPosition, len(frameData))
	for i, fd := range frameData {
		images[i] = fd.Image
		positions[i] = fd.Cursor
	}

	return images, positions, nil
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
		frames := captureWaitFrames(page, currentCursor, action.Duration, frameInterval)
		return frames, currentCursor, nil
	case "navigate":
		page.MustNavigate(action.URL)
		page.MustWaitLoad()
		frame, _ := captureFrame(page)
		return []FrameData{{Image: frame, Cursor: currentCursor}}, currentCursor, nil
	default:
		return nil, currentCursor, fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeClickAnimated performs a click with cursor movement animation
func executeClickAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	el, err := page.Element(action.Selector)
	if err != nil {
		return nil, currentCursor, fmt.Errorf("element not found: %s", action.Selector)
	}

	x, y, err := getElementCenter(el)
	if err != nil {
		return nil, currentCursor, err
	}

	var frames []FrameData

	// Animate cursor movement to target (over ~0.5 seconds)
	movementFrames := opts.FPS / 2
	if movementFrames < 5 {
		movementFrames = 5
	}

	for i := 0; i <= movementFrames; i++ {
		t := float64(i) / float64(movementFrames)
		t = easeInOutQuad(t) // Smooth easing

		interpX := int(float64(currentCursor.X) + t*(float64(x)-float64(currentCursor.X)))
		interpY := int(float64(currentCursor.Y) + t*(float64(y)-float64(currentCursor.Y)))

		// Move actual mouse
		page.Mouse.MustMoveTo(float64(interpX), float64(interpY))

		frame, err := captureFrame(page)
		if err != nil {
			continue
		}

		cursor := CursorPosition{X: interpX, Y: interpY, State: CursorPointer}
		frames = append(frames, FrameData{Image: frame, Cursor: cursor})

		time.Sleep(frameInterval / 2) // Faster for movement
	}

	// Perform actual click
	el.MustClick()

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
		frames = append(frames, FrameData{Image: frame, Cursor: cursor})
		time.Sleep(frameInterval)
	}

	return frames, CursorPosition{X: x, Y: y, State: CursorPointer}, nil
}

// executeTypeAnimated performs typing with character-by-character animation
func executeTypeAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	el, err := page.Element(action.Selector)
	if err != nil {
		return nil, currentCursor, fmt.Errorf("element not found: %s", action.Selector)
	}

	x, y, err := getElementCenter(el)
	if err != nil {
		return nil, currentCursor, err
	}

	var frames []FrameData

	// Animate cursor movement to input field
	movementFrames := opts.FPS / 2
	if movementFrames < 5 {
		movementFrames = 5
	}

	for i := 0; i <= movementFrames; i++ {
		t := float64(i) / float64(movementFrames)
		t = easeInOutQuad(t)

		interpX := int(float64(currentCursor.X) + t*(float64(x)-float64(currentCursor.X)))
		interpY := int(float64(currentCursor.Y) + t*(float64(y)-float64(currentCursor.Y)))

		page.Mouse.MustMoveTo(float64(interpX), float64(interpY))

		frame, err := captureFrame(page)
		if err != nil {
			continue
		}

		cursor := CursorPosition{X: interpX, Y: interpY, State: CursorText}
		frames = append(frames, FrameData{Image: frame, Cursor: cursor})

		time.Sleep(frameInterval / 2)
	}

	// Click to focus
	el.MustClick()

	// Clear existing text
	el.MustSelectAllText()

	// Capture frame after focus
	frame, _ := captureFrame(page)
	frames = append(frames, FrameData{
		Image:  frame,
		Cursor: CursorPosition{X: x, Y: y, State: CursorText},
	})

	// Type character by character
	text := action.Text
	typingDelay := 50 * time.Millisecond // 50ms between characters
	frameEvery := 2                       // Capture frame every N characters

	for i, char := range text {
		// Type the character
		page.Keyboard.MustType(input.Key(char))

		// Capture frame every few characters
		if i%frameEvery == 0 || i == len(text)-1 {
			time.Sleep(typingDelay)
			frame, err := captureFrame(page)
			if err != nil {
				continue
			}
			frames = append(frames, FrameData{
				Image:  frame,
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
			Cursor: CursorPosition{X: x, Y: y, State: CursorText},
		})
		time.Sleep(frameInterval)
	}

	return frames, CursorPosition{X: x, Y: y, State: CursorText}, nil
}

// executeScrollAnimated performs scroll with animation
func executeScrollAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	var frames []FrameData

	scrollSteps := 10
	stepX := float64(action.X) / float64(scrollSteps)
	stepY := float64(action.Y) / float64(scrollSteps)

	for i := 0; i < scrollSteps; i++ {
		page.Mouse.MustScroll(stepX, stepY)
		time.Sleep(frameInterval)

		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		frames = append(frames, FrameData{Image: frame, Cursor: currentCursor})
	}

	return frames, currentCursor, nil
}

// executeHoverAnimated performs hover with cursor movement animation
func executeHoverAnimated(page *rod.Page, action Action, currentCursor CursorPosition, opts Options, frameInterval time.Duration) ([]FrameData, CursorPosition, error) {
	el, err := page.Element(action.Selector)
	if err != nil {
		return nil, currentCursor, fmt.Errorf("element not found: %s", action.Selector)
	}

	x, y, err := getElementCenter(el)
	if err != nil {
		return nil, currentCursor, err
	}

	var frames []FrameData

	// Animate cursor movement
	movementFrames := opts.FPS / 2
	for i := 0; i <= movementFrames; i++ {
		t := float64(i) / float64(movementFrames)
		t = easeInOutQuad(t)

		interpX := int(float64(currentCursor.X) + t*(float64(x)-float64(currentCursor.X)))
		interpY := int(float64(currentCursor.Y) + t*(float64(y)-float64(currentCursor.Y)))

		page.Mouse.MustMoveTo(float64(interpX), float64(interpY))

		frame, err := captureFrame(page)
		if err != nil {
			continue
		}

		cursor := CursorPosition{X: interpX, Y: interpY, State: CursorPointer}
		frames = append(frames, FrameData{Image: frame, Cursor: cursor})

		time.Sleep(frameInterval / 2)
	}

	// Trigger hover
	el.MustHover()

	// Capture hover state
	for i := 0; i < opts.FPS/4; i++ {
		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		frames = append(frames, FrameData{
			Image:  frame,
			Cursor: CursorPosition{X: x, Y: y, State: CursorPointer},
		})
		time.Sleep(frameInterval)
	}

	return frames, CursorPosition{X: x, Y: y, State: CursorPointer}, nil
}

// captureWaitFrames captures frames during a wait period
func captureWaitFrames(page *rod.Page, cursor CursorPosition, waitMs int, frameInterval time.Duration) []FrameData {
	var frames []FrameData

	numFrames := waitMs / int(frameInterval.Milliseconds())
	if numFrames < 1 {
		numFrames = 1
	}
	if numFrames > 60 { // Cap at ~3 seconds worth
		numFrames = 60
	}

	for i := 0; i < numFrames; i++ {
		frame, err := captureFrame(page)
		if err != nil {
			continue
		}
		frames = append(frames, FrameData{Image: frame, Cursor: cursor})
		time.Sleep(frameInterval)
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
