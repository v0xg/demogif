# Roadmap

## Phase 1: Reliability ✅

- [x] Missing selectors no longer hang the run; failed actions are skipped instead of panicking
- [x] Non-ASCII text (accents, emoji) types correctly
- [x] `--fps` validated (1–100)
- [x] GIF playback speed matches real time (delays from capture timestamps)
- [x] Radio groups / shared `name` attributes get unique selectors
- [x] A failed action stops the batch, re-crawls, and is reported to the AI as `FAILED`
- [x] `wait` actions no longer wait twice
- [x] Per-frame palettes (later pages no longer dithered against the first page's colors)
- [x] Palette sort is O(n log n) and deterministic
- [x] Frames are encoded incrementally on background workers instead of held in memory

## Phase 2: Safety net

- [x] Tests: `parseActionsJSON`, cursor drawing, crawler and executor against a local page in headless Chromium
- [x] CI on pull requests and `main` (gofmt, build, `go vet`, `go test -race`)
- [x] Fix the `go vet` warning and remove dead code (`GetElementType`, `GetElementPosition`)
- [ ] Remove remaining `Must*` calls in the crawler; always close the browser on error
- [ ] Timeouts and retries on AI API calls; update the default Claude model
- [x] Align Go version: README, CI and release all follow `go.mod`

## Phase 3: Smarter automation

- [ ] Vision: send a screenshot with the page map
- [ ] New actions: `select` (dropdowns) and `press` (Enter, Escape, …)
- [ ] `--dry-run` to print the plan; `--save-script` / `--replay` for deterministic, AI-free re-recording

## Phase 4: Better output

- [ ] Smaller GIFs: merge duplicate frames, encode only changed regions
- [ ] `--output-width` (currently fixed at 800) and MP4/WebM output
- [ ] Start the cursor at the real viewport center instead of a fixed 640×360
- [ ] Click highlight / zoom and optional captions
