package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"github.com/v0xg/demogif/internal/ai"
	"github.com/v0xg/demogif/internal/crawler"
	"github.com/v0xg/demogif/internal/executor"
	"github.com/v0xg/demogif/internal/gifgen"
	"github.com/v0xg/demogif/internal/overlay"
)

var (
	output    string
	fps       int
	width     int
	height    int
	delay     int
	provider  string
	model     string
	noCursor  bool
	verbose   bool
	profile   string
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
	execOpts := executor.Options{
		FPS:       fps,
		BaseDelay: delay,
		Verbose:   verbose,
	}

	var allFrames []image.Image
	var allCursors []executor.CursorPosition
	var completedActions []executor.Action
	var lastCursor *executor.CursorPosition

	// Capture initial hold frames
	initialFrames, initialCursors := captureHoldFrames(browser, fps, nil)
	allFrames = append(allFrames, initialFrames...)
	allCursors = append(allCursors, initialCursors...)

	// Agentic loop: execute until checkpoint, re-crawl, continue
	maxIterations := 20 // Safety limit
	iteration := 0

	for len(actions) > 0 && iteration < maxIterations {
		iteration++

		// Execute current batch of actions
		result, err := executor.ExecuteBatch(browser, actions, execOpts, lastCursor)
		if err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}

		allFrames = append(allFrames, result.Frames...)
		allCursors = append(allCursors, result.CursorPositions...)
		lastCursor = &result.LastCursor

		// Track completed actions for context
		if result.HitCheckpoint {
			completedActions = append(completedActions, actions[:result.CheckpointIndex+1]...)
		} else {
			completedActions = append(completedActions, actions...)
		}

		// If we hit a checkpoint, re-crawl and ask AI to continue
		if result.HitCheckpoint {
			fmt.Printf("→ Checkpoint reached, re-analyzing page... ")
			pageMap, err = browser.ReCrawl()
			if err != nil {
				fmt.Println("failed")
				return fmt.Errorf("re-crawl failed: %w", err)
			}
			fmt.Printf("done (found %d elements)\n", len(pageMap.Elements))

			// Ask AI to continue
			fmt.Printf("→ Continuing action generation... ")
			completedSummary := formatCompletedActions(completedActions)
			actions, err = aiProvider.ContinueActions(pageMap, prompt, completedSummary)
			if err != nil {
				fmt.Println("failed")
				return fmt.Errorf("continue generation failed: %w", err)
			}
			fmt.Printf("done (%d actions)\n", len(actions))
			logActions(actions)
		} else {
			// No checkpoint, we're done
			actions = nil
		}
	}

	if iteration >= maxIterations {
		fmt.Println("⚠ Max iterations reached, stopping")
	}

	// Capture final hold frames
	finalFrames, finalCursors := captureHoldFrames(browser, fps, lastCursor)
	allFrames = append(allFrames, finalFrames...)
	allCursors = append(allCursors, finalCursors...)

	// Step 4: Apply cursor overlay
	if !noCursor {
		fmt.Printf("→ Applying cursor overlay... ")
		allFrames, err = overlay.ApplyCursor(allFrames, allCursors)
		if err != nil {
			fmt.Println("failed")
			return fmt.Errorf("overlay failed: %w", err)
		}
		fmt.Println("done")
	}

	// Step 5: Generate GIF
	fmt.Printf("→ Generating GIF (%d frames)... ", len(allFrames))
	gifOpts := gifgen.Options{
		FPS:      fps,
		MaxWidth: 800,
	}
	fileSize, err := gifgen.Generate(allFrames, output, gifOpts)
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

// formatCompletedActions creates a summary of completed actions for the AI
func formatCompletedActions(actions []executor.Action) string {
	var lines []string
	for i, action := range actions {
		switch action.Type {
		case "type":
			lines = append(lines, fmt.Sprintf("%d. Typed %q into %s", i+1, action.Text, action.Selector))
		case "click":
			lines = append(lines, fmt.Sprintf("%d. Clicked %s", i+1, action.Selector))
		case "navigate":
			lines = append(lines, fmt.Sprintf("%d. Navigated to %s", i+1, action.URL))
		case "hover":
			lines = append(lines, fmt.Sprintf("%d. Hovered over %s", i+1, action.Selector))
		case "scroll":
			lines = append(lines, fmt.Sprintf("%d. Scrolled by (%d, %d)", i+1, action.X, action.Y))
		case "wait":
			lines = append(lines, fmt.Sprintf("%d. Waited %dms", i+1, action.Duration))
		}
	}
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result
}

// captureHoldFrames captures frames for hold periods (start/end of GIF)
func captureHoldFrames(browser *crawler.Browser, targetFPS int, cursor *executor.CursorPosition) ([]image.Image, []executor.CursorPosition) {
	page := browser.Page()
	numFrames := targetFPS // 1 second worth

	defaultCursor := executor.CursorPosition{X: 640, Y: 360, State: executor.CursorDefault}
	if cursor != nil {
		defaultCursor = *cursor
	}

	var frames []image.Image
	var cursors []executor.CursorPosition

	for i := 0; i < numFrames; i++ {
		data, err := page.Screenshot(false, nil)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		frames = append(frames, img)
		cursors = append(cursors, defaultCursor)
	}

	return frames, cursors
}

func logVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}
