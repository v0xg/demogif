# demogif

CLI tool to generate animated GIFs of web interactions using AI.

## Install

```bash
go install github.com/v0xg/demogif/cmd/demogif@latest
```

Or build from source:

```bash
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
