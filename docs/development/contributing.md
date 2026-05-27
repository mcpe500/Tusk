# Contributing to Tusk

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/Tusk.git
   cd Tusk
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/mcpe500/Tusk.git
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/my-feature
# or
git checkout -b fix/issue-number
```

### 2. Make Changes

Follow Go conventions:
- Run `go fmt` before committing
- Run `go vet` to check for issues
- Write tests for new functionality

### 3. Test Changes

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Test specific package
go test ./internal/vm/...
```

### 4. Commit

Follow conventional commits:

```
feat: add container exec command
fix: resolve QMP connection timeout
docs: update README with new commands
refactor: simplify image store interface
```

### 5. Push and Create PR

```bash
git push origin feature/my-feature
# Then create PR via GitHub UI
```

## Code Style

- Use `gofmt` for formatting
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Keep functions small and focused
- Document exported functions

## Project Structure

```
Tusk/
├── cmd/              # Entry points (tusk, tuskd)
├── internal/         # Internal packages
│   ├── vm/          # VM management
│   ├── image/       # Image store
│   ├── client/      # RPC client
│   └── compose/     # Docker compose
├── pkg/             # Public packages
└── docs/            # Documentation
```

## Reporting Issues

- Check existing issues first
- Include system info (Termux version, Go version)
- Include error messages and stack traces
- Provide minimal reproduction steps

---

*Back to [docs](../README.md)*