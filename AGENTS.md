# AI Coding Agent Guidelines

## Repository Overview

This is a Go-based CLI application called MHAI (Modular Hybrid AI) that provides an interface for interacting with various AI providers. The codebase follows standard Go conventions with a modular structure.

## Build, Lint, and Test Commands

### Build Commands
```bash
# Build the binary to build/bin
make build

# Clean build artifacts
make clean

# Install the binary (requires sudo)
make install

# Uninstall the binary (requires sudo)
make uninstall

# Show help
make help
```

### Testing Commands
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Run a specific test package
go test ./internal/db

# Run a specific test function
go test ./internal/db -run TestFunctionName

# Run tests with verbose output
go test ./... -v

# Run race detector
go test ./... -race
```

### Linting and Formatting
```bash
# Format code (if no specific linter is configured)
go fmt ./...

# Vet for suspicious constructs
go vet ./...

# Static analysis
staticcheck ./...  # if installed

# Security checks
gosec ./...  # if installed
```

## Code Style Guidelines

### File Organization
- Each package should have a clear, single responsibility
- Group related functionality in internal/ packages
- Main package only contains application entrypoint
- cmd/ package contains command-line interface logic

### Import Organization
1. Standard library imports (alphabetical)
2. Blank line
3. External imports (alphabetical by host)
4. Blank line
5. Local/project imports (alphabetical)

Example:
```go
import (
    "database/sql"
    "fmt"
    "os"
    "path/filepath"

    _ "github.com/glebarez/go-sqlite"

    "github.com/mhai-org/mhai/internal/db"
    "github.com/mhai-org/mhai/internal/security"
)
```

### Naming Conventions
- Use mixedCaps for variables, functions, constants, and types
- Package names should be lowercase, single words
- Exported names start with uppercase letter
- Interface names ending in -er when they have one method (e.g., Reader, Writer)
- Struct fields use mixedCaps
- Constants use mixedCaps or ALL_CAPS for truly constant values
- Error variables should be named err or descriptive names like validationErr
- Context parameters should be named ctx

### Error Handling
- Always check errors returned from functions
- Handle errors at the appropriate level (don't just ignore)
- Return errors early when they prevent normal function execution
- Wrap errors with fmt.Errorf("%w", err) when adding context
- Use sentinel errors for expected error conditions
- Log errors appropriately (typically at the boundary where they're handled)
- Don't log and return the same error (choose one)

Example from codebase:
```go
if err := db.Ping(); err != nil {
    return nil, err
}

// Later in the same function:
return &Database{Conn: db}, nil
```

### Structs and Interfaces
- Use struct tags for serialization/deserialization when needed
- Keep interfaces small and focused
- Prefer composition over inheritance
- Initialize structs using composite literals
- Don't expose internal state unnecessarily

### Control Flow
- Use early returns to reduce nesting
- Keep functions focused and reasonably sized
- Use blank lines to separate logical sections within functions
- Defer cleanup operations immediately after acquiring resources

### Comments
- Comment exported functions, types, and constants
- Use full sentences in comments
- Comment why, not what (unless the what is non-obvious)
- Avoid commenting bad code - rewrite it instead
- TODO comments should include relevant context

### Specific Patterns Observed in Codebase
- Database connections use sql.Open with proper error handling
- File paths constructed using filepath.Join
- Directory creation with os.MkdirAll and proper permissions
- SQL queries use parameterized statements to prevent injection
- Encryption uses proper IV generation and secure modes
- Context is used appropriately for cancellation (in newer code)

### Testing Guidelines
- Table-driven tests for functions with multiple input/output cases
- Test both positive and negative cases
- Mock external dependencies when appropriate
- Focus on behavior, not implementation details
- Keep tests focused and independent
- Use descriptive test names that explain what is being tested
- Avoid testing private functions directly; test through public interface

### Security Considerations
- Never hardcode secrets or keys
- Use environment variables or secure vaults for credentials
- Encrypt sensitive data at rest (as seen in security/ package)
- Use parameterized queries to prevent SQL injection
- Validate and sanitize user input
- Use appropriate cryptographic primitives and modes

## Additional Notes

### Version Control
- Write clear, descriptive commit messages
- Keep commits focused on a single change
- Reference issues in commit messages when applicable
- Follow conventional commits format when possible

### Documentation
- Godoc comments for all exported entities
- Keep documentation updated with code changes
- Examples in documentation should be compilable
- README should contain setup and usage instructions
