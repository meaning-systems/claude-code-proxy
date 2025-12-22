# claude-code-proxy

OpenAI-compatible API proxy for Claude Code CLI. Use your Claude Code Max subscription for inference instead of paying for API credits.

> **⚠️ Disclaimer:** As of December 2024, this is compatible with Anthropic's Terms of Service for personal use. TOS may change. This repo is not maintained. See [LICENSE](LICENSE). **Use at your own risk.**

## Quick Start

```bash
# Prerequisites: Claude Code CLI authenticated, Go installed

# Run
PROXY_API_KEY=your-secret go run main.go
```

Configure your app:

| Setting | Value |
|---------|-------|
| **Endpoint** | `http://localhost:8080/v1/chat/completions` |
| **Model** | `claude-haiku` (or anything) |
| **API Key** | `your-secret` |

## Run as a Background Service

See **[CLAUDE.md](CLAUDE.md)** for detailed setup instructions on:
- macOS (launchd)
- Linux (systemd)
- Windows (Task Scheduler)

## Configuration

| Env Variable | Default | Options |
|--------------|---------|---------|
| `PROXY_API_KEY` | (required) | Any string |
| `PORT` | `8080` | Any port |
| `CLAUDE_MODEL` | `haiku` | `haiku`, `sonnet`, `opus` |

## How It Works

```
Your App  →  localhost:8080  →  claude --print  →  Response
```

The proxy receives OpenAI-format requests, pipes them to the Claude CLI, and returns OpenAI-format responses.

## License

[Unlicense](LICENSE) (public domain) — but read the Anthropic TOS notice in the license file.
