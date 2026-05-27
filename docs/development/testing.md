# Testing

## Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test -v ./internal/vm/...
```

## Test Categories

### Unit Tests

Tests for individual functions and packages.

```bash
go test ./pkg/...
go test ./internal/...
```

### Integration Tests

Tests that require VM or other external resources.

```bash
# Requires VM to be running
TUSK_TEST_VM=1 go test -tags=integration ./...
```

## Test Structure

```
Tusk/
├── cmd/
│   └── tusk/
│       └── main_test.go    # CLI tests
├── internal/
│   ├── vm/
│   │   ├── manager_test.go
│   │   └── qmp_test.go
│   ├── image/
│   │   └── store_test.go
│   └── compose/
│       └── parser_test.go
└── pkg/
    └── types/
        └── types_test.go
```

## Writing Tests

```go
package vm_test

import (
    "testing"
    "github.com/tusk/tusk/internal/vm"
)

func TestNewManager(t *testing.T) {
    mgr := vm.New("/tmp/test")
    if mgr == nil {
        t.Error("Expected manager, got nil")
    }
}

func TestVMStatus(t *testing.T) {
    mgr := vm.New("/tmp/test")
    status := mgr.Status()
    if status != vm.StatusStopped {
        t.Errorf("Expected StatusStopped, got %s", status)
    }
}
```

## Testing Infrastructure

### Mock tuskd

For testing without VM, use simulation mode:

```bash
echo -e "ping\ninfo\nexit" | ./tuskd
```

### Mock QMP Server

For testing QMP client:

```bash
# Start a mock QMP server
socat - UNIX-LISTEN:/tmp/test-qmp.sock,fork
```

## CI/CD

Tests run on every push via GitHub Actions:

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Run tests
        run: go test -v ./...
```

## Coverage

Generate coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

*Back to [docs](../README.md)*