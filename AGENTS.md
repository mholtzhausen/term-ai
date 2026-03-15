# MHAI — AI Coding Agent Guidelines

**Module:** `github.com/mhai-org/mhai` | **Binary:** `ai` | **Go:** 1.25.6

MHAI is a Go CLI/TUI tool for interacting with OpenAI-compatible AI providers. SQLite stores conversations, providers, and personas locally at `~/.mhai/mhai.db`.

## Build and Test

```bash
make build          # → build/bin/ai
make clean          # rm -rf build/
make install        # sudo ln -sf to /usr/local/bin/ai
make uninstall      # sudo rm from /usr/local/bin/ai

go test ./...               # run all tests (none exist yet)
go test ./... -cover -race  # with coverage and race detector
go fmt ./...                # format
go vet ./...                # suspicious constructs
```

> There are currently **no test files** in the repo. When adding tests, use table-driven patterns and mock external dependencies (DB, API).

## Architecture

Three execution paths in `cmd/root.go`:

1. **Interactive TUI** (`ai` with no args) → `ui.LaunchInteractive()` — full-screen Bubble Tea app
2. **Direct CLI** (`ai "prompt"`) → `ai.StreamChatWithHistory()` — streams response to stdout; renders with glamour
3. **Auto-resume** — if a `cli`-platform conversation is <5 min old, it is automatically continued

**Component boundaries:**

| Package | Responsibility |
|---------|---------------|
| `cmd/` | Cobra CLI commands; wires internal packages together |
| `internal/ai/` | OpenAI-compatible HTTP client (SSE streaming), conversation CRUD |
| `internal/config/` | Provider storage with encrypted API keys; active provider/model config keys |
| `internal/db/` | SQLite connection + schema init (4 tables) |
| `internal/persona/` | Persona CRUD; default persona is `"You are a helpful assistant."` |
| `internal/security/` | AES-256-CFB encrypt/decrypt for API keys |
| `internal/ui/` | Bubble Tea TUI (`interactive.go`), direct-mode status bar (`direct.go`), command palette (`palette.go`) |
| `internal/utils/` | `writer.go` — dual `io.Writer` for token counting |

**DB schema:** `providers`, `personas`, `config`, `conversations`
- `conversations.history` is a JSON-encoded `[]ai.Message`
- `conversations.platform` is `"cli"` or `"tui"`
- API keys in `providers` are AES-encrypted before storage

**TUI key bindings:** `Ctrl+P` — command palette | `Enter` — send | `Esc` — close modal

## Key Files

- `cmd/root.go` — three-way dispatch; persona `@name` resolution; auto-resume logic
- `internal/ai/client.go` — SSE streaming; 60-second request timeout; models endpoint derived from chat URL
- `internal/ai/conversation.go` — time parsing handles 3 formats (SQLite datetime, RFC3339, RFC3339Z)
- `internal/security/encryption.go` — **hardcoded master key** (known issue; see Security below)
- `internal/ui/interactive.go` — 750+ line Bubble Tea model; provider wizard is a 3-step modal

## Code Style

### Imports (three groups, alphabetical within each)
```go
import (
    "database/sql"
    "fmt"

    _ "github.com/glebarez/go-sqlite"

    "github.com/mhai-org/mhai/internal/db"
    "github.com/mhai-org/mhai/internal/security"
)
```

### Error handling
- Return errors early; wrap with `fmt.Errorf("context: %w", err)` when adding context
- Don't both log and return the same error — choose one
- Use sentinel errors for expected conditions

### Naming
- `mixedCaps` everywhere; package names lowercase single words
- Error vars named `err` or descriptive (e.g., `validationErr`); context params named `ctx`
- Single-method interfaces end in `-er` (e.g., `Writer`)

### Patterns
- Parameterized SQL everywhere — never string-concatenate queries
- `filepath.Join` for all paths; `os.MkdirAll` with `0755` for dirs
- Defer cleanup immediately after acquiring resources
- Chat messages use `INSERT OR REPLACE` via conversation ID

## Pitfalls

- **Hardcoded encryption key**: `internal/security/encryption.go` uses a hardcoded 32-byte `masterKey`. Moving it to an env var or keyring would be a breaking change for existing DBs.
- **No tests**: All `go test` commands will succeed vacuously. Don't assume test infrastructure exists when writing new code.
- **Model URL derivation**: `ListModels` strips `/chat/completions` to build the `/models` URL — non-standard provider URLs may break this.
- **Conversation time parsing**: Three date formats are tried in sequence; adding a new DB source must match one of them.
- **TUI is stateful and large**: `internal/ui/interactive.go` has a large `model` struct with 10+ fields. Read it carefully before adding TUI features.
- **Binary is named `ai`**: The installed binary clashes with any existing `ai` command on `$PATH`.

## Security

- **Never hardcode new secrets** — the existing master key is a known technical debt, don't add more
- All SQL queries must use parameterized statements (`$1` / `?` placeholders)
- API keys are encrypted at rest via `security.Encrypt()` before any DB write
- Validate user input at command boundaries before passing to internal packages

## Version Control

- Conventional Commits format: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`
- Keep commits focused on a single change; reference issues when applicable
