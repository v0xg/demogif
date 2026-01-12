package crawler

// PageMap represents the analyzed structure of a web page
type PageMap struct {
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	Elements   []Element `json:"elements"`
	Navigation []NavItem `json:"navigation"`
	IsSPA      bool      `json:"isSPA"`
}

// Element represents an interactive element on the page
type Element struct {
	Selector    string `json:"selector"`
	Type        string `json:"type"` // button, input, link, select, checkbox, radio
	Text        string `json:"text,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Name        string `json:"name,omitempty"`
	ID          string `json:"id,omitempty"`
}

// NavItem represents a navigation link
type NavItem struct {
	Selector string `json:"selector"`
	Text     string `json:"text"`
	Href     string `json:"href"`
}
