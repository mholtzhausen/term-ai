# MHAI (Modular Hybrid AI)

MHAI is a minimalist, beautiful, and powerful Golang-based CLI/TUI tool designed for rich interaction with various AI providers (Anthropic, OpenAI, etc.). It prioritizes speed, aesthetics, and a modular architecture.

> [!CAUTION]
> **DISCLAIMER:** This project was coded **99% by AI** (Antigravity). Use this software at your own risk. The authors are not responsible for any data loss, API costs, or unexpected behavior.

---

## 🚀 Features

- **Multi-Modal Personas**: Use `@name` to switch between custom personas with specific system prompts.
- **Interactive TUI**: A rich terminal interface built with Charm libraries (Bubble Tea, Lip Gloss).
- **Command Palette**: Access settings, providers, and conversation history via `Ctrl+P`.
- **Session Persistence**: Conversations are saved to a local SQLite database and can be resumed across sessions.
- **Auto-Resumption**: CLI mode automatically resumes recent conversations (less than 5 minutes old) to maintain context.
- **Provider Management**: Easily switch between Anthropic, OpenAI, and other OpenAI-compatible APIs.

## 🛠️ Installation

### Quick Install (Linux)

You can install MHAI directly using the following command:

```bash
curl -fsSL https://raw.githubusercontent.com/mholtzhausen/term-ai/main/scripts/install.sh | sh
```

*Note: This script requires `go`, `git`, and `make` to be installed on your system.*

### Manual Build

1. Clone the repository:
   ```bash
   git clone https://github.com/mholtzhausen/term-ai.git
   cd term-ai
   ```
2. Build and install:
   ```bash
   make build
   sudo make install
   ```

## 📖 Usage

### Interactive TUI Mode
Simply run the command to enter the full-screen interactive mode:
```bash
ai
```

### Direct CLI Mode
Send a single prompt and get a response:
```bash
ai "What is the capital of France?"
```

### Using Personas
Invoke a specific persona using the `@` prefix:
```bash
ai @coder "Refactor this function..."
```

### Configuration
Manage your AI providers:
```bash
# Set a new provider
ai config set-provider --name anthropic --key YOUR_KEY --url https://api.anthropic.com/v1/messages
```

## ⌨️ TUI Shortcuts

- `Ctrl+P`: Open Command Palette
- `Enter`: Send message
- `Esc`: Exit / Close modal
- `Up/Down`: Scroll history

---

Built with ❤️ by AI for humans.
