package crawler

import (
	"testing"

	"github.com/v0xg/demogif/internal/testutil"
)

const formPage = `<html><body>
<form>
  <label><input type="radio" name="size" value="s">S</label>
  <label><input type="radio" name="size" value="m">M</label>
  <label><input type="radio" name="size" value="l">L</label>
  <input name="email" placeholder="Email">
  <input type="hidden" name="token" value="x">
  <button type="button" class="btn primary">Save</button>
  <button type="button" class="btn">Cancel</button>
  <button type="button" class="btn">Cancel too</button>
</form>
<a href="/next">Next</a>
<div style="display:none"><button id="hidden-btn">Hidden</button></div>
</body></html>`

func TestCrawlSelectorsAreUnique(t *testing.T) {
	testutil.RequireBrowser(t)

	pageMap, browser, err := Crawl(testutil.ServeHTML(t, formPage), Options{Width: 800, Height: 600})
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	byType := map[string]int{}
	for _, el := range pageMap.Elements {
		byType[el.Type]++
		count := browser.Page().MustEval(`(sel) => document.querySelectorAll(sel).length`, el.Selector).Int()
		if count != 1 {
			t.Errorf("selector %q (%s) matches %d elements, want 1", el.Selector, el.Type, count)
		}
		if el.Selector == "#hidden-btn" {
			t.Errorf("hidden element was extracted")
		}
	}

	want := map[string]int{"radio": 3, "button": 3, "link": 1}
	for typ, n := range want {
		if byType[typ] != n {
			t.Errorf("found %d %s elements, want %d (all: %+v)", byType[typ], typ, n, pageMap.Elements)
		}
	}
}
