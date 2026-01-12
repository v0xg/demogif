# demogif

CLI tool to generate animated GIFs of web interactions using AI.

## Install

### Homebrew (macOS/Linux)

```bash
brew install v0xg/tap/demogif
```

### Download Binary

Download the appropriate archive for your system from the [releases page](https://github.com/v0xg/demogif/releases):

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `demogif_*_darwin_arm64.tar.gz` |
| macOS (Intel) | `demogif_*_darwin_amd64.tar.gz` |
| Linux (x64) | `demogif_*_linux_amd64.tar.gz` |
| Linux (ARM64) | `demogif_*_linux_arm64.tar.gz` |
| Windows (x64) | `demogif_*_windows_amd64.zip` |

Then extract and move to your PATH:

```bash
# macOS/Linux
tar xzf demogif_*.tar.gz
sudo mv demogif /usr/local/bin/

# Windows (PowerShell)
Expand-Archive demogif_*.zip -DestinationPath .
Move-Item demogif.exe C:\Windows\System32\
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
