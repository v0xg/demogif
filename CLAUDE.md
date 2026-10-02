# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`demogif` is a Go CLI that turns a URL + natural-language prompt into an animated GIF: it crawls the page with headless Chromium (go-rod), asks an LLM for a JSON action script, replays the actions while capturing screenshots, draws a synthetic cursor, and encodes a GIF.

## Commands

```bash
go build -o demogif ./cmd/demogif          # build (binary is gitignored)
go run ./cmd/demogif "<url>" "<prompt>" -v # run with verbose logging
go vet ./...
go test ./...                              # only internal/gifgen has tests so far; single test: go test ./internal/gifgen -run TestFrameDelays
```

Running requires a Chrome/Chromium binary (found via `launcher.LookPath`) and an API key: `ANTHROPIC_API_KEY` (or `DEMOGIF_ANTHROPIC_KEY`) for Claude, `OPENAI_API_KEY` (or `DEMOGIF_OPENAI_KEY`) for OpenAI. `DEMOGIF_DEFAULT_PROVIDER` picks the provider when `--provider` is omitted. A `.env` file is auto-loaded via godotenv.

Releases: pushing a `v*` tag runs GoReleaser (`.goreleaser.yaml`), which builds cross-platform binaries and updates the `v0xg/homebrew-tap` formula. `main.version` is set via ldflags.

## Architecture

The whole pipeline is orchestrated in `cmd/demogif/main.go` `run()`; packages under `internal/` don't call each other except through shared types (`crawler.PageMap`, `crawler.Browser`, `executor.Action`, `executor.CursorPosition`).

1. **crawler** — launches the browser (optionally with `--profile` user-data dir for logged-in sessions), waits for load/network idle (extra wait for detected SPAs), and extracts interactive elements + navigation into a `PageMap` via in-page JS. Returns a `*Browser` that stays open for the rest of the run; `Browser.ReCrawl()` rebuilds the `PageMap` from the current page state.
2. **ai** — `Provider` interface (`GenerateActions`, `ContinueActions`) with Claude and OpenAI implementations. Both share the prompts in `prompt.go` and the tolerant JSON extractor `parseActionsJSON` in `claude.go`. Default models are set in each constructor.
3. **executor** — `ExecuteBatch` performs actions (`click`, `type`, `scroll`, `hover`, `wait`, `navigate`) with eased cursor movement, capturing timestamped `FrameData` (screenshot + `CursorPosition`) and streaming them to `Options.OnFrame` after each action. It stops after the first `checkpoint` action or the first failure (`StoppedAt`/`Err`). Element lookups are bounded by `elementTimeout`; avoid rod `Must*` calls in action code, since a panic kills the whole run.
4. **overlay** — cursor is not captured by screenshots; `DrawCursor` draws a pixel-drawn cursor and click ripple onto a single frame.
5. **gifgen** — `Encoder.Add` downscales (max width 800, hardcoded in main) and quantizes each frame with its own palette on a background worker pool, so full-size screenshots are never all held in memory and capture timing isn't skewed. `Write` derives each frame's delay from real capture timestamps (capped at 1s so AI/re-crawl pauses don't freeze the GIF).

### Checkpoint loop (key design)

The LLM is instructed to emit actions only **up to and including the first action that changes the page significantly** (modal open, route change, submit), marked `"checkpoint": true`. When a batch stops at a checkpoint or a failed action, `main.go` re-crawls, sends the new `PageMap` plus a numbered summary of completed steps (failed ones prefixed `FAILED:`) to `ContinueActions`, and repeats until a batch runs to completion, the model returns `[]`, or 20 iterations are hit. This is how multi-page flows work without the model guessing selectors that don't exist yet. Changes to action semantics need to be kept in sync across `executor.Action` (JSON tags), the system prompt in `ai/prompt.go`, `logActions`/`describeAction` in main, and the executor's action switch.

Viewport-centre defaults (640,360) are hardcoded for the initial cursor in both main and executor, independent of `--width/--height`.
