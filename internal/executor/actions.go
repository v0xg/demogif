package executor

// Action represents a single browser automation action
type Action struct {
	Type     string `json:"action"`             // click, type, scroll, hover, wait, navigate
	Selector string `json:"selector,omitempty"` // CSS selector for the target element
	Text     string `json:"text,omitempty"`     // Text to type (for type action)
	X        int    `json:"x,omitempty"`        // X coordinate (for scroll)
	Y        int    `json:"y,omitempty"`        // Y coordinate (for scroll)
	URL      string `json:"url,omitempty"`      // URL for navigate action
	Duration int    `json:"wait,omitempty"`     // Wait duration in ms after action
}

// CursorPosition represents the cursor state at a point in time
type CursorPosition struct {
	X      int
	Y      int
	State  CursorState
	Click  bool // Whether a click happened at this position
}

// CursorState represents the visual state of the cursor
type CursorState int

const (
	CursorDefault CursorState = iota
	CursorPointer
	CursorText
)
