# Claude Code Proxy — Setup Guide

This guide will help you set up claude-code-proxy to run automatically on your computer. Once set up, any app that supports custom AI providers can use your Claude Code subscription.

## What This Does

```
┌─────────────┐       ┌─────────────────┐       ┌─────────────┐
│   Your App  │  ───► │ claude-code-    │  ───► │ Claude Code │
│ (any app    │       │ proxy           │       │ CLI         │
│  that works │  ◄─── │ (localhost:8080)│  ◄─── │ (your sub)  │
│  with AI)   │       └─────────────────┘       └─────────────┘
└─────────────┘
```

Your app sends requests to `localhost:8080`, the proxy translates them and sends them to Claude Code CLI, which uses your authenticated subscription.

---

## Prerequisites

Before starting, you need:

1. **Claude Code CLI** installed and authenticated
   - Install: https://docs.anthropic.com/en/docs/claude-code
   - Run `claude` once in your terminal to authenticate

2. **Go** (the programming language) to build the proxy
   - macOS: `brew install go`
   - Linux: `sudo apt install golang` or `sudo dnf install golang`
   - Windows: Download from https://go.dev/dl/

### Verify Claude Code Works

Open a terminal and run:
```bash
echo "Say hello" | claude --print
```

If you see a response from Claude, you're ready to continue.

---

## Quick Test (Before Setting Up as Daemon)

First, let's make sure everything works:

```bash
# 1. Download or clone this repo
git clone https://github.com/YOUR_USERNAME/claude-code-proxy.git
cd claude-code-proxy

# 2. Run the proxy (replace "your-secret" with any password you want)
PROXY_API_KEY=your-secret go run main.go
```

In another terminal, test it:
```bash
curl http://localhost:8080/health
# Should print: ok
```

If that works, you're ready to set it up as a permanent service.

---

## Setting Up as a Daemon (Auto-Start)

A "daemon" is a program that runs in the background and starts automatically when your computer boots. Choose your operating system below.

### 📱 macOS

macOS uses **launchd** to manage background services.

#### Step 1: Build the Binary

```bash
cd claude-code-proxy
go build -o claude-code-proxy main.go
```

This creates an executable file called `claude-code-proxy`.

#### Step 2: Move It Somewhere Permanent

```bash
# Create a folder for it
mkdir -p ~/daemons/claude-code-proxy

# Move the files
cp claude-code-proxy ~/daemons/claude-code-proxy/
```

#### Step 3: Create the Service File

Create a file at `~/Library/LaunchAgents/com.claude-code-proxy.plist`:

```bash
nano ~/Library/LaunchAgents/com.claude-code-proxy.plist
```

Paste this (change YOUR_USERNAME to your actual username, and change the API key):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.claude-code-proxy</string>

    <key>ProgramArguments</key>
    <array>
        <string>/Users/YOUR_USERNAME/daemons/claude-code-proxy/claude-code-proxy</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>PROXY_API_KEY</key>
        <string>your-secret-key-here</string>
        <key>PORT</key>
        <string>8080</string>
        <key>CLAUDE_MODEL</key>
        <string>haiku</string>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin</string>
        <key>HOME</key>
        <string>/Users/YOUR_USERNAME</string>
    </dict>

    <key>RunAtLoad</key>
    <true/>

    <key>KeepAlive</key>
    <true/>

    <key>StandardOutPath</key>
    <string>/Users/YOUR_USERNAME/daemons/claude-code-proxy/proxy.log</string>

    <key>StandardErrorPath</key>
    <string>/Users/YOUR_USERNAME/daemons/claude-code-proxy/error.log</string>
</dict>
</plist>
```

Save and exit (in nano: Ctrl+O, Enter, Ctrl+X).

#### Step 4: Start the Service

```bash
launchctl load ~/Library/LaunchAgents/com.claude-code-proxy.plist
```

#### Step 5: Verify It's Running

```bash
curl http://localhost:8080/health
# Should print: ok
```

#### macOS Commands Reference

```bash
# Check if running
launchctl list | grep claude-code-proxy

# View logs
tail -f ~/daemons/claude-code-proxy/proxy.log

# Stop the service
launchctl unload ~/Library/LaunchAgents/com.claude-code-proxy.plist

# Start the service
launchctl load ~/Library/LaunchAgents/com.claude-code-proxy.plist

# Restart (stop then start)
launchctl unload ~/Library/LaunchAgents/com.claude-code-proxy.plist
launchctl load ~/Library/LaunchAgents/com.claude-code-proxy.plist
```

---

### 🐧 Linux

Linux uses **systemd** to manage background services.

#### Step 1: Build the Binary

```bash
cd claude-code-proxy
go build -o claude-code-proxy main.go
```

#### Step 2: Move It Somewhere Permanent

```bash
mkdir -p ~/daemons/claude-code-proxy
cp claude-code-proxy ~/daemons/claude-code-proxy/
```

#### Step 3: Create the Service File

```bash
mkdir -p ~/.config/systemd/user
nano ~/.config/systemd/user/claude-code-proxy.service
```

Paste this (change YOUR_USERNAME and the API key):

```ini
[Unit]
Description=Claude Code Proxy
After=network.target

[Service]
Type=simple
Environment="PROXY_API_KEY=your-secret-key-here"
Environment="PORT=8080"
Environment="CLAUDE_MODEL=haiku"
Environment="HOME=/home/YOUR_USERNAME"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"
ExecStart=/home/YOUR_USERNAME/daemons/claude-code-proxy/claude-code-proxy
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
```

Save and exit.

#### Step 4: Enable and Start

```bash
# Reload systemd to see the new service
systemctl --user daemon-reload

# Enable it to start on boot
systemctl --user enable claude-code-proxy

# Start it now
systemctl --user start claude-code-proxy

# Enable lingering (so it runs even when you're not logged in)
loginctl enable-linger $USER
```

#### Step 5: Verify It's Running

```bash
curl http://localhost:8080/health
# Should print: ok
```

#### Linux Commands Reference

```bash
# Check status
systemctl --user status claude-code-proxy

# View logs
journalctl --user -u claude-code-proxy -f

# Stop
systemctl --user stop claude-code-proxy

# Start
systemctl --user start claude-code-proxy

# Restart
systemctl --user restart claude-code-proxy

# Disable from starting on boot
systemctl --user disable claude-code-proxy
```

---

### 🪟 Windows

Windows uses **Task Scheduler** or can run as a Windows Service.

#### Step 1: Build the Binary

Open PowerShell or Command Prompt:

```powershell
cd claude-code-proxy
go build -o claude-code-proxy.exe main.go
```

#### Step 2: Move It Somewhere Permanent

```powershell
mkdir C:\daemons\claude-code-proxy
copy claude-code-proxy.exe C:\daemons\claude-code-proxy\
```

#### Step 3: Create a Startup Script

Create `C:\daemons\claude-code-proxy\start.bat`:

```batch
@echo off
set PROXY_API_KEY=your-secret-key-here
set PORT=8080
set CLAUDE_MODEL=haiku
C:\daemons\claude-code-proxy\claude-code-proxy.exe
```

#### Step 4: Add to Task Scheduler

1. Press `Win + R`, type `taskschd.msc`, press Enter
2. Click "Create Basic Task..." in the right panel
3. Name: `Claude Code Proxy`
4. Trigger: "When the computer starts"
5. Action: "Start a program"
6. Program: `C:\daemons\claude-code-proxy\start.bat`
7. Check "Open Properties dialog" before finishing
8. In Properties:
   - Check "Run whether user is logged on or not"
   - Check "Run with highest privileges"
   - In "Settings" tab, uncheck "Stop the task if it runs longer than"

#### Step 5: Start It Now

Right-click the task and select "Run".

#### Step 6: Verify It's Running

Open PowerShell:
```powershell
curl http://localhost:8080/health
# Should print: ok
```

#### Windows Commands Reference

```powershell
# Check if running (in PowerShell)
Get-Process claude-code-proxy -ErrorAction SilentlyContinue

# View what's listening on port 8080
netstat -an | findstr 8080

# Stop (kill the process)
Stop-Process -Name claude-code-proxy

# Start (run the task)
# Use Task Scheduler GUI or:
schtasks /run /tn "Claude Code Proxy"
```

---

## Configuring Your App

Once the proxy is running, configure your app with these settings:

| Setting | Value |
|---------|-------|
| **API Endpoint URL** | `http://localhost:8080/v1/chat/completions` |
| **Model Name** | `claude-haiku` (or any name — it's ignored) |
| **API Key** | Whatever you set in `PROXY_API_KEY` |

---

## Choosing a Model

You can change which Claude model the proxy uses by setting the `CLAUDE_MODEL` environment variable.

### Available Models (as of December 2024)

> **⚠️ Note for future readers:** Anthropic updates their models regularly. The models below were available when this was written. If a model doesn't work, check Claude Code's current supported models by running `claude --help` or visiting the Claude Code documentation.

| Alias | Full Model ID | Speed | Best For |
|-------|---------------|-------|----------|
| `haiku` | claude-haiku-4-5-20241022 | ⚡ Fastest | Quick tasks, high volume |
| `sonnet` | claude-sonnet-4-5-20241022 | 🔄 Balanced | General use |
| `opus` | claude-opus-4-5-20251101 | 🧠 Smartest | Complex reasoning |

You can use either the **alias** (e.g., `haiku`) or the **full model ID**.

### How to Change the Model

#### Option 1: Environment Variable (Recommended)

Set `CLAUDE_MODEL` when starting the proxy:

```bash
CLAUDE_MODEL=sonnet PROXY_API_KEY=your-secret go run main.go
```

#### Option 2: Update Your Service Config

**macOS** — Edit the plist file:
```xml
<key>CLAUDE_MODEL</key>
<string>sonnet</string>  <!-- Change this line -->
```
Then restart: `launchctl unload ... && launchctl load ...`

**Linux** — Edit the service file:
```ini
Environment="CLAUDE_MODEL=sonnet"  # Change this line
```
Then restart: `systemctl --user restart claude-code-proxy`

**Windows** — Edit `start.bat`:
```batch
set CLAUDE_MODEL=sonnet
```
Then restart the task in Task Scheduler.

### Testing a Different Model

Before updating your service config, test the model works:

```bash
echo "Hello" | claude --print --model sonnet
```

If you get a response, the model is available.

---

## Troubleshooting

### "Connection refused" when testing

The proxy isn't running. Check:
- Is it started? (see commands for your OS above)
- Check the logs for errors

### "Claude CLI failed" in logs

Claude Code CLI isn't working. Try:
```bash
echo "test" | claude --print
```

If this fails, re-authenticate Claude Code by running `claude` in your terminal.

### "Invalid API key" response

The API key in your app doesn't match `PROXY_API_KEY`. Make sure they're identical.

### Proxy stops working after computer restart

The service isn't set up correctly. Re-follow the steps for your OS, especially:
- macOS: Make sure the plist is in `~/Library/LaunchAgents/`
- Linux: Make sure you ran `systemctl --user enable` and `loginctl enable-linger`
- Windows: Make sure the task is set to run "When the computer starts"

---

## For AI Agents

If you're an AI agent reading this to help a user:

### Setup Checklist

1. **Check prerequisites first** — verify `claude --print` works before anything else
2. **Use absolute paths** — the daemon configs need full paths, not `~` or relative paths
3. **Match the username** — replace `YOUR_USERNAME` with the actual system username
4. **Test before daemonizing** — always run `go run main.go` first to verify it works
5. **Check logs on failure** — the log paths are specified in each service config

### Important: Model Availability May Change

**This documentation was written in December 2024.** Anthropic regularly updates Claude Code and the available models. If you're reading this in the future:

1. **Model aliases may change** — `haiku`, `sonnet`, `opus` might map to different model versions
2. **New models may exist** — There might be newer, better options not listed here
3. **Old models may be deprecated** — Some model IDs might no longer work

**To check current available models:**
```bash
claude --help
# Look for the --model flag documentation
```

**To test if a model works:**
```bash
echo "test" | claude --print --model MODEL_NAME
```

If the proxy fails with "Claude CLI failed" errors after an update, the model name in `CLAUDE_MODEL` may need to be updated to a currently supported model.

### Code Structure

The proxy is ~200 lines of Go with no dependencies. Key parts:
- `handleChat()` — receives requests, calls Claude, returns responses
- `CLAUDE_MODEL` env var — passed to `claude --print --model`
- OpenAI-compatible request/response format

If something's wrong, the code is simple enough to debug directly.
