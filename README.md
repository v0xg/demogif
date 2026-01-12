# demogif

CLI tool to generate animated GIFs of web interactions using AI.

## Install

### Homebrew (macOS/Linux)

```bash
brew install v0xg/tap/demogif
```

### Go Install

Requires Go 1.23+:

```bash
go install github.com/v0xg/demogif/cmd/demogif@latest
```

### Build from Source

```bash
git clone https://github.com/v0xg/demogif.git
cd demogif
go build -o demogif ./cmd/demogif
```

## Usage

```bash
demogif "https://example.com" "click the button, fill the form"
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `demo.gif` | Output filename |
| `--fps` | `20` | Frames per second |
| `--width` | `1280` | Viewport width |
| `--height` | `720` | Viewport height |
| `--delay` | `800` | Base delay between actions (ms) |
| `--provider` | `claude` | AI provider: `claude` or `openai` |
| `--model` | - | Specific model override |
| `--no-cursor` | `false` | Disable cursor overlay |
| `-v, --verbose` | `false` | Show detailed progress |

## Configuration

Set in environment or `.env` file:
- `ANTHROPIC_API_KEY` - Required for Claude (default provider)
- `OPENAI_API_KEY` - Required for OpenAI provider
