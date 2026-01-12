# demogif

CLI tool to generate animated GIFs of web interactions using AI.

## Install

```bash
go build -o demogif ./cmd/demogif
```

## Usage

```bash
./demogif "https://example.com" "click the button, fill the form"
```

**Flags:** `-o` output, `--fps`, `--width`, `--height`, `--delay`, `--provider` (claude/openai), `--model`, `--no-cursor`, `-v` verbose

## Configuration

Set in environment or `.env` file:
- `ANTHROPIC_API_KEY` - Required for Claude (default provider)
- `OPENAI_API_KEY` - Required for OpenAI provider
