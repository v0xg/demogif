package crawler

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// Options configures the crawler behavior
type Options struct {
	Width      int
	Height     int
	Timeout    time.Duration
	Verbose    bool
	ProfileDir string // Chrome/Chromium profile directory for authenticated sessions
}

// Browser wraps the Rod browser and page for reuse
type Browser struct {
	browser *rod.Browser
	page    *rod.Page
}

// Close cleans up browser resources
func (b *Browser) Close() {
	if b.page != nil {
		b.page.Close()
	}
	if b.browser != nil {
		b.browser.Close()
	}
}

// Page returns the underlying Rod page
func (b *Browser) Page() *rod.Page {
	return b.page
}

// ReCrawl extracts a fresh PageMap from the current browser page state
// Used after checkpoint actions to get updated elements
func (b *Browser) ReCrawl() (*PageMap, error) {
	page := b.page

	// Wait for any pending navigation/content to settle
	page.MustWaitLoad()

	// Wait for network idle with timeout (don't hang on persistent connections)
	page.Timeout(5 * time.Second).WaitRequestIdle(500*time.Millisecond, nil, nil, nil)()

	// Wait for interactive elements to appear (SPAs need time to render new content)
	waitForInteractiveElements(page, 5*time.Second)

	// Extract current URL and title
	url := page.MustEval(`() => window.location.href`).String()
	title := page.MustEval(`() => document.title`).String()

	// Detect if SPA
	isSPA := detectSPA(page)

	// Extract interactive elements
	elements := extractElements(page)

	// Extract navigation
	navigation := extractNavigation(page)

	return &PageMap{
		URL:        url,
		Title:      title,
		Elements:   elements,
		Navigation: navigation,
		IsSPA:      isSPA,
	}, nil
}

// Crawl navigates to a URL and extracts page structure
func Crawl(url string, opts Options) (*PageMap, *Browser, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	// Launch headless browser
	path, _ := launcher.LookPath()
	l := launcher.New().Bin(path).Headless(true)

	if opts.ProfileDir != "" {
		l = l.UserDataDir(opts.ProfileDir)
	}

	u := l.MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()

	page := browser.MustPage(url)

	// Set viewport
	page.MustSetViewport(opts.Width, opts.Height, 1, false)

	// Wait for page load
	page.MustWaitLoad()

	// Wait for network to be idle (important for SPAs)
	// Use timeout to avoid hanging on persistent connections (WebSockets, polling, etc.)
	page.Timeout(5 * time.Second).WaitRequestIdle(500*time.Millisecond, nil, nil, nil)()

	// Detect if SPA
	isSPA := detectSPA(page)

	// If SPA, wait for interactive elements to appear (Next.js/React apps need time to
	// download JS bundles, hydrate, and potentially fetch client-side data)
	if isSPA {
		waitForInteractiveElements(page, 5*time.Second)
	}

	// Extract page info
	title := page.MustEval(`() => document.title`).String()

	// Extract interactive elements
	elements := extractElements(page)

	// Extract navigation
	navigation := extractNavigation(page)

	pageMap := &PageMap{
		URL:        url,
		Title:      title,
		Elements:   elements,
		Navigation: navigation,
		IsSPA:      isSPA,
	}

	return pageMap, &Browser{browser: browser, page: page}, nil
}

// waitForInteractiveElements polls until interactive elements appear or timeout
func waitForInteractiveElements(page *rod.Page, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	checkInterval := 200 * time.Millisecond

	for time.Now().Before(deadline) {
		count := page.MustEval(`() => {
			const buttons = document.querySelectorAll('button, [role="button"], input[type="submit"]');
			const inputs = document.querySelectorAll('input:not([type="hidden"]), textarea');
			const links = document.querySelectorAll('a[href]');
			let visible = 0;
			buttons.forEach(el => { if (el.offsetParent) visible++; });
			inputs.forEach(el => { if (el.offsetParent) visible++; });
			links.forEach(el => { if (el.offsetParent) visible++; });
			return visible;
		}`).Int()

		if count > 0 {
			// Found elements, wait a tiny bit more for any final renders
			time.Sleep(300 * time.Millisecond)
			return
		}

		time.Sleep(checkInterval)
	}
}

// detectSPA checks if the page is a Single Page Application
func detectSPA(page *rod.Page) bool {
	// Check for common SPA framework markers
	result := page.MustEval(`() => {
		// React
		if (window.__REACT_DEVTOOLS_GLOBAL_HOOK__ || document.querySelector('[data-reactroot]') || document.querySelector('#__next')) return true;
		// Vue
		if (window.__VUE__ || document.querySelector('[data-v-]')) return true;
		// Angular
		if (window.ng || document.querySelector('[ng-version]') || document.querySelector('app-root')) return true;
		// Svelte
		if (document.querySelector('[class*="svelte-"]')) return true;
		return false;
	}`)
	return result.Bool()
}

// extractElements finds interactive elements on the page
func extractElements(page *rod.Page) []Element {
	result := page.MustEval(`() => {
		const elements = [];
		const seen = new Set();

		// Helper to check if a class name is a valid CSS identifier
		function isValidCSSClass(cls) {
			// CSS class names can't start with a digit or hyphen followed by digit
			// Also avoid classes with special chars like . : [ ] etc.
			if (!cls || cls.length === 0) return false;
			if (/^[0-9]/.test(cls)) return false;
			if (/^-[0-9]/.test(cls)) return false;
			if (/[.:#\[\]()>~+*\/\\]/.test(cls)) return false;
			return true;
		}

		// Helper to generate unique selector
		function getSelector(el) {
			if (el.id && isValidCSSClass(el.id)) return '#' + el.id;
			if (el.name) return '[name="' + el.name + '"]';

			// Use class-based selector (filter out invalid CSS class names)
			if (el.className && typeof el.className === 'string') {
				const validClasses = el.className.trim().split(/\s+/).filter(isValidCSSClass).slice(0, 2);
				if (validClasses.length > 0) {
					const selector = el.tagName.toLowerCase() + '.' + validClasses.join('.');
					try {
						if (document.querySelectorAll(selector).length === 1) {
							return selector;
						}
					} catch (e) {
						// Invalid selector, fall through
					}
				}
			}

			// Fallback to nth-child
			const parent = el.parentElement;
			if (parent) {
				const siblings = Array.from(parent.children);
				const index = siblings.indexOf(el) + 1;
				const parentSelector = getSelector(parent);
				if (parentSelector) {
					return parentSelector + ' > ' + el.tagName.toLowerCase() + ':nth-child(' + index + ')';
				}
			}

			return el.tagName.toLowerCase();
		}

		// Buttons
		document.querySelectorAll('button, [role="button"], input[type="submit"], input[type="button"]').forEach(el => {
			if (!el.offsetParent) return; // Not visible
			const selector = getSelector(el);
			if (seen.has(selector)) return;
			seen.add(selector);
			elements.push({
				selector: selector,
				type: 'button',
				text: (el.textContent || el.value || '').trim().slice(0, 50),
				id: el.id || undefined,
				name: el.name || undefined
			});
		});

		// Input fields
		document.querySelectorAll('input:not([type="hidden"]):not([type="submit"]):not([type="button"]), textarea').forEach(el => {
			if (!el.offsetParent) return;
			const selector = getSelector(el);
			if (seen.has(selector)) return;
			seen.add(selector);
			elements.push({
				selector: selector,
				type: el.type || 'text',
				placeholder: el.placeholder || undefined,
				id: el.id || undefined,
				name: el.name || undefined
			});
		});

		// Links (only visible, non-navigation)
		document.querySelectorAll('a[href]').forEach(el => {
			if (!el.offsetParent) return;
			const href = el.getAttribute('href');
			if (href.startsWith('#') || href.startsWith('javascript:')) return;
			const selector = getSelector(el);
			if (seen.has(selector)) return;
			seen.add(selector);
			elements.push({
				selector: selector,
				type: 'link',
				text: (el.textContent || '').trim().slice(0, 50),
				id: el.id || undefined
			});
		});

		// Select dropdowns
		document.querySelectorAll('select').forEach(el => {
			if (!el.offsetParent) return;
			const selector = getSelector(el);
			if (seen.has(selector)) return;
			seen.add(selector);
			elements.push({
				selector: selector,
				type: 'select',
				id: el.id || undefined,
				name: el.name || undefined
			});
		});

		// Checkboxes and radios
		document.querySelectorAll('input[type="checkbox"], input[type="radio"]').forEach(el => {
			if (!el.offsetParent) return;
			const selector = getSelector(el);
			if (seen.has(selector)) return;
			seen.add(selector);
			elements.push({
				selector: selector,
				type: el.type,
				id: el.id || undefined,
				name: el.name || undefined
			});
		});

		return elements;
	}`)

	var elements []Element
	for _, v := range result.Arr() {
		el := Element{
			Selector:    v.Get("selector").String(),
			Type:        v.Get("type").String(),
			Text:        v.Get("text").String(),
			Placeholder: v.Get("placeholder").String(),
			Name:        v.Get("name").String(),
			ID:          v.Get("id").String(),
		}
		elements = append(elements, el)
	}

	return elements
}

// extractNavigation finds navigation links
func extractNavigation(page *rod.Page) []NavItem {
	result := page.MustEval(`() => {
		const navItems = [];
		const seen = new Set();

		// Look in nav elements first
		document.querySelectorAll('nav a, header a, [role="navigation"] a').forEach(el => {
			if (!el.offsetParent) return;
			const href = el.getAttribute('href');
			if (!href || href === '#' || href.startsWith('javascript:')) return;
			if (seen.has(href)) return;
			seen.add(href);

			let selector = '';
			if (el.id) {
				selector = '#' + el.id;
			} else {
				selector = 'a[href="' + href + '"]';
			}

			navItems.push({
				selector: selector,
				text: (el.textContent || '').trim().slice(0, 30),
				href: href
			});
		});

		return navItems;
	}`)

	var navItems []NavItem
	for _, v := range result.Arr() {
		item := NavItem{
			Selector: v.Get("selector").String(),
			Text:     v.Get("text").String(),
			Href:     v.Get("href").String(),
		}
		navItems = append(navItems, item)
	}

	return navItems
}

// GetElementPosition returns the center position of an element
func GetElementPosition(page *rod.Page, selector string) (x, y int, err error) {
	el, err := page.Element(selector)
	if err != nil {
		return 0, 0, fmt.Errorf("element not found: %s", selector)
	}

	box, err := el.Shape()
	if err != nil {
		return 0, 0, err
	}

	if len(box.Quads) == 0 {
		return 0, 0, fmt.Errorf("element has no shape: %s", selector)
	}

	// Get center of first quad
	quad := box.Quads[0]
	centerX := (quad[0] + quad[2] + quad[4] + quad[6]) / 4
	centerY := (quad[1] + quad[3] + quad[5] + quad[7]) / 4

	return int(centerX), int(centerY), nil
}

// GetElementType determines cursor type for an element
func GetElementType(page *rod.Page, selector string) string {
	result := page.MustEval(fmt.Sprintf(`(selector) => {
		const el = document.querySelector(selector);
		if (!el) return 'default';
		const tag = el.tagName.toLowerCase();
		const type = el.type || '';
		if (tag === 'input' && (type === 'text' || type === 'email' || type === 'password' || type === 'search' || type === 'tel' || type === 'url' || type === '')) return 'text';
		if (tag === 'textarea') return 'text';
		if (tag === 'a' || tag === 'button' || el.getAttribute('role') === 'button') return 'pointer';
		return 'default';
	}`, escapeSelector(selector)))

	return result.String()
}

func escapeSelector(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}
