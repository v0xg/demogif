package executor

import (
	"strings"
	"testing"

	"github.com/v0xg/demogif/internal/crawler"
	"github.com/v0xg/demogif/internal/testutil"
)

const actionPage = `<html><body>
<input id="q" style="width:300px">
<textarea id="t"></textarea>
<button id="b" onclick="document.title='clicked'">Go</button>
<button id="other" onclick="document.title='other'">Other</button>
</body></html>`

func startPage(t *testing.T) *crawler.Browser {
	t.Helper()
	testutil.RequireBrowser(t)
	_, browser, err := crawler.Crawl(testutil.ServeHTML(t, actionPage), crawler.Options{Width: 800, Height: 600})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(browser.Close)
	return browser
}

func run(t *testing.T, browser *crawler.Browser, actions []Action) (*ExecuteResult, []FrameData) {
	t.Helper()
	var frames []FrameData
	opts := Options{FPS: 20, BaseDelay: 50, OnFrame: func(f FrameData) { frames = append(frames, f) }}
	return ExecuteBatch(browser, actions, opts, nil), frames
}

func eval(browser *crawler.Browser, js string) string {
	return browser.Page().MustEval(js).String()
}

func TestExecuteBatchRunsActions(t *testing.T) {
	browser := startPage(t)

	res, frames := run(t, browser, []Action{
		{Type: "type", Selector: "#q", Text: "héllo ñ 🎉"},
		{Type: "type", Selector: "#t", Text: "a\nb"},
		{Type: "hover", Selector: "#b"},
		{Type: "click", Selector: "#b"},
	})

	if res.StoppedAt != -1 || res.Err != nil {
		t.Fatalf("StoppedAt=%d Err=%v, want a full run", res.StoppedAt, res.Err)
	}
	if got := eval(browser, `() => document.getElementById('q').value`); got != "héllo ñ 🎉" {
		t.Errorf("input value = %q", got)
	}
	if got := eval(browser, `() => document.getElementById('t').value`); got != "a\nb" {
		t.Errorf("textarea value = %q", got)
	}
	if got := eval(browser, `() => document.title`); got != "clicked" {
		t.Errorf("title = %q, click did not fire", got)
	}

	if len(frames) == 0 {
		t.Fatal("no frames emitted")
	}
	for i, f := range frames {
		if f.Image == nil {
			t.Fatalf("frame %d has nil image", i)
		}
		if i > 0 && f.At.Before(frames[i-1].At) {
			t.Fatalf("frame %d timestamp goes backwards", i)
		}
	}
	if last := res.LastCursor; last.X == 0 && last.Y == 0 {
		t.Error("cursor position was not tracked")
	}
}

func TestExecuteBatchStopsAtCheckpoint(t *testing.T) {
	browser := startPage(t)

	res, _ := run(t, browser, []Action{
		{Type: "click", Selector: "#b", Checkpoint: true},
		{Type: "click", Selector: "#other"},
	})

	if res.StoppedAt != 0 || res.Err != nil {
		t.Fatalf("StoppedAt=%d Err=%v, want stop at checkpoint 0", res.StoppedAt, res.Err)
	}
	if got := eval(browser, `() => document.title`); got != "clicked" {
		t.Errorf("title = %q, action after checkpoint should not run", got)
	}
}

func TestExecuteBatchStopsOnMissingElement(t *testing.T) {
	browser := startPage(t)

	res, _ := run(t, browser, []Action{
		{Type: "click", Selector: "#does-not-exist"},
		{Type: "click", Selector: "#b"},
	})

	if res.StoppedAt != 0 || res.Err == nil || !strings.Contains(res.Err.Error(), "element not found") {
		t.Fatalf("StoppedAt=%d Err=%v, want element-not-found at 0", res.StoppedAt, res.Err)
	}
	if got := eval(browser, `() => document.title`); got == "clicked" {
		t.Error("actions after the failure should not run")
	}
}

func TestExecuteBatchUnknownAction(t *testing.T) {
	browser := startPage(t)

	res, _ := run(t, browser, []Action{{Type: "teleport"}})
	if res.StoppedAt != 0 || res.Err == nil {
		t.Fatalf("StoppedAt=%d Err=%v, want failure on unknown action", res.StoppedAt, res.Err)
	}
}
