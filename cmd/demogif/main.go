package main

import (
	"fmt"
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
		Width:   width,
		Height:  height,
		Verbose: verbose,
	}
	pageMap, browser, err := crawler.Crawl(url, crawlerOpts)
	if err != nil {
		fmt.Println("failed")
		return fmt.Errorf("crawl failed: %w", err)
	}
	fmt.Printf("done (found %d interactive elements)\n", len(pageMap.Elements))

	// Step 2: Generate actions via AI
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

	// Step 3: Execute actions and capture frames
	fmt.Println("→ Recording...")
	execOpts := executor.Options{
		FPS:       fps,
		BaseDelay: delay,
		Verbose:   verbose,
	}
	frames, cursorPositions, err := executor.Execute(browser, actions, execOpts)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}

	// Step 4: Apply cursor overlay
	if !noCursor {
		fmt.Printf("→ Applying cursor overlay... ")
		frames, err = overlay.ApplyCursor(frames, cursorPositions)
		if err != nil {
			fmt.Println("failed")
			return fmt.Errorf("overlay failed: %w", err)
		}
		fmt.Println("done")
	}

	// Step 5: Generate GIF
	fmt.Printf("→ Generating GIF (%d frames)... ", len(frames))
	gifOpts := gifgen.Options{
		FPS:      fps,
		MaxWidth: 800,
	}
	fileSize, err := gifgen.Generate(frames, output, gifOpts)
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

func logVerbose(format string, args ...interface{}) {
	if verbose {
		fmt.Printf(format+"\n", args...)
	}
}
