package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/v0xg/demogif/internal/ai"
	"github.com/v0xg/demogif/internal/crawler"
	"github.com/v0xg/demogif/internal/executor"
	"github.com/v0xg/demogif/internal/gifgen"
	"github.com/v0xg/demogif/internal/overlay"
)

var (
	output   string
	fps      int
	width    int
	height   int
	delay    int
	provider string
	model    string
	noCursor bool
	verbose  bool
	profile  string
)

func main() {
	// Load .env file if present (silently ignore if not found)
	_ = godotenv.Load()

	rootCmd := &cobra.Command{
		Use:   "demogif <url> <prompt>",
		Short: "Generate polished GIFs of web app demos using AI",
		Long: `demogif crawls a website, uses AI to generate browser automation scripts
based on your natural language prompt, and creates a polished GIF demo.

Example:
  demogif "https://myapp.com" "click login, fill email with test@example.com, submit form"`,
		Args: cobra.ExactArgs(2),
		RunE: run,
	}

	rootCmd.Flags().StringVarP(&output, "output", "o", "demo.gif", "Output filename")
	rootCmd.Flags().IntVar(&fps, "fps", 20, "Frames per second")
	rootCmd.Flags().IntVar(&width, "width", 1280, "Viewport width")
	rootCmd.Flags().IntVar(&height, "height", 720, "Viewport height")
	rootCmd.Flags().IntVar(&delay, "delay", 800, "Base delay between actions (ms)")
	rootCmd.Flags().StringVar(&provider, "provider", "", "AI provider: claude, openai (default: from env or claude)")
	rootCmd.Flags().StringVar(&model, "model", "", "Specific model override")
	rootCmd.Flags().BoolVar(&noCursor, "no-cursor", false, "Disable cursor overlay")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed progress")
	rootCmd.Flags().StringVar(&profile, "profile", "", "Chrome/Chromium profile directory for authenticated sessions (close browser first)")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	url := args[0]
	prompt := args[1]

	// GIF frame delays are in 1/100s, so above 100 FPS the delay rounds to 0;
	// FPS <= 0 would divide by zero when computing frame intervals
	if fps < 1 || fps > 100 {
		return fmt.Errorf("--fps must be between 1 and 100 (got %d)", fps)
	}

	// Determine AI provider
	selectedProvider := provider
	if selectedProvider == "" {
		selectedProvider = os.Getenv("DEMOGIF_DEFAULT_PROVIDER")
		if selectedProvider == "" {
			selectedProvider = "claude"
		}
	}

	logVerbose("Starting demogif")
	logVerbose("  URL: %s", url)
	logVerbose("  Prompt: %s", prompt)
	logVerbose("  Provider: %s", selectedProvider)

	// Step 1: Crawl the page
	fmt.Printf("→ Crawling %s... ", url)
	crawlerOpts := crawler.Options{
		Width:      width,
		Height:     height,
		Verbose:    verbose,
		ProfileDir: profile,
	}
	pageMap, browser, err := crawler.Crawl(url, crawlerOpts)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("crawl failed: %w", err)
	}
	fmt.Printf("done (found %d interactive elements)\n", len(pageMap.Elements))

	// Step 2: Generate initial actions via AI
	fmt.Printf("→ Generating action script via %s... ", selectedProvider)
	aiProvider, err := ai.NewProvider(selectedProvider, model)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("AI provider init failed: %w", err)
	}
	actions, err := aiProvider.GenerateActions(pageMap, prompt)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("action generation failed: %w", err)
	}
	fmt.Printf("done (%d actions)\n", len(actions))
	logActions(actions)

	// Step 3: Execute actions with checkpoint-based re-crawling
	fmt.Println("→ Recording...")
	// Frames are cursor-overlaid and encoded as they arrive, so the full-size
	// screenshots never accumulate in memory
	enc := gifgen.NewEncoder(800, fps)
	record := func(f executor.FrameData) {
		img := f.Image
		if !noCursor {
			img = overlay.DrawCursor(img, f.Cursor)
		}
		enc.Add(img, f.At)
	}

	execOpts := executor.Options{
		FPS:       fps,
		BaseDelay: delay,
		Verbose:   verbose,
		OnFrame:   record,
	}

	var completedSteps []string
	var lastCursor *executor.CursorPosition

	// Capture initial hold frames
	captureHoldFrames(browser, fps, nil, record)

	// Agentic loop: execute until checkpoint or failure, re-crawl, continue
	maxIterations := 20 // Safety limit
	iteration := 0

	for len(actions) > 0 && iteration < maxIterations {
		iteration++

		// Execute current batch of actions
		result := executor.ExecuteBatch(browser, actions, execOpts, lastCursor)
		lastCursor = &result.LastCursor

		// Track what ran (and what failed) as context for the AI
		ran := actions
		if result.StoppedAt >= 0 {
			ran = actions[:result.StoppedAt+1]
		}
		for i, action := range ran {
			if result.Err != nil && i == len(ran)-1 {
				completedSteps = append(completedSteps, fmt.Sprintf("FAILED: %s (%v)", describeAction(action), result.Err))
			} else {
				completedSteps = append(completedSteps, describeAction(action))
			}
		}

		if result.StoppedAt < 0 {
			// Ran every action without a checkpoint, we're done
			break
		}

		// Checkpoint or failure: re-crawl and ask AI to continue
		if result.Err != nil {
			fmt.Printf("→ Action failed (%v), re-analyzing page... ", result.Err)
		} else {
			fmt.Printf("→ Checkpoint reached, re-analyzing page... ")
		}
		pageMap, err = browser.ReCrawl()
		if err != nil {
			fmt.Println("failed")
			return fmt.Errorf("re-crawl failed: %w", err)
		}
		fmt.Printf("done (found %d elements)\n", len(pageMap.Elements))

		fmt.Printf("→ Continuing action generation... ")
		actions, err = aiProvider.ContinueActions(pageMap, prompt, formatCompletedSteps(completedSteps))
		if err != nil {
			fmt.Println("failed")
			return fmt.Errorf("continue generation failed: %w", err)
		}
		fmt.Printf("done (%d actions)\n", len(actions))
		logActions(actions)
	}

	if iteration >= maxIterations && len(actions) > 0 {
		fmt.Println("⚠ Max iterations reached, stopping")
	}

	// Capture final hold frames
	captureHoldFrames(browser, fps, lastCursor, record)

	// Step 4: Write GIF
	fmt.Printf("→ Writing GIF (%d frames)... ", enc.Len())
	fileSize, err := enc.Write(output)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("GIF generation failed: %w", err)
	}
	fmt.Println("done")

	// Cleanup
	browser.Close()

	fmt.Printf("✓ Saved to %s (%.1f MB)\n", output, float64(fileSize)/(1024*1024))
	return nil
}

// logActions prints the action list
func logActions(actions []executor.Action) {
	for i, action := range actions {
		checkpoint := ""
		if action.Checkpoint {
			checkpoint = " [checkpoint]"
		}
		switch action.Type {
		case "type":
			fmt.Printf("  [%d] %s → %s (text: %q)%s\n", i+1, action.Type, action.Selector, action.Text, checkpoint)
		case "wait":
			fmt.Printf("  [%d] %s → %dms%s\n", i+1, action.Type, action.Duration, checkpoint)
		case "navigate":
			fmt.Printf("  [%d] %s → %s%s\n", i+1, action.Type, action.URL, checkpoint)
		default:
			fmt.Printf("  [%d] %s → %s%s\n", i+1, action.Type, action.Selector, checkpoint)
		}
	}
}

// describeAction summarizes an action for the AI's completed-steps context
func describeAction(action executor.Action) string {
	switch action.Type {
	case "type":
		return fmt.Sprintf("Typed %q into %s", action.Text, action.Selector)
	case "click":
		return fmt.Sprintf("Clicked %s", action.Selector)
	case "navigate":
		return fmt.Sprintf("Navigated to %s", action.URL)
	case "hover":
		return fmt.Sprintf("Hovered over %s", action.Selector)
	case "scroll":
		return fmt.Sprintf("Scrolled by (%d, %d)", action.X, action.Y)
	case "wait":
		return fmt.Sprintf("Waited %dms", action.Duration)
	default:
		return fmt.Sprintf("%s %s", action.Type, action.Selector)
	}
}

// formatCompletedSteps numbers the completed steps for the AI
func formatCompletedSteps(steps []string) string {
	var b strings.Builder
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}
	return b.String()
}

// captureHoldFrames records ~1 second of still frames (start/end of GIF)
func captureHoldFrames(browser *crawler.Browser, targetFPS int, cursor *executor.CursorPosition, record func(executor.FrameData)) {
	page := browser.Page()

	defaultCursor := executor.CursorPosition{X: 640, Y: 360, State: executor.CursorDefault}
	if cursor != nil {
		defaultCursor = *cursor
	}

	interval := time.Second / time.Duration(targetFPS)
	deadline := time.Now().Add(time.Second)
	for {
		next := time.Now().Add(interval)
		data, err := page.Screenshot(false, nil)
		if err == nil {
			if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
				record(executor.FrameData{Image: img, Cursor: defaultCursor, At: time.Now()})
			}
		}
		if !time.Now().Before(deadline) {
			return
		}
		time.Sleep(time.Until(next))
	}
}

func logVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}
