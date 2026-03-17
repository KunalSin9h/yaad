# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make build          # Build binary to bin/lore
make install        # go install ./cmd/lore
make test           # go test ./...

go test ./internal/app/...                          # Run service tests only
go test ./internal/adapters/sqlite/...              # Run SQLite adapter tests only
go test -run TestMemoryService_Add ./internal/app/  # Run a single test

bash scripts/test_cli.sh                            # End-to-end CLI integration tests
bash scripts/test_cli.sh --no-build                 # E2E tests without rebuilding
```

Lint: `golangci-lint run` (config in `.golangci.yml`)

## Architecture

Lore is a **Ports & Adapters (Hexagonal)** app. Business logic never imports infrastructure.

```
cmd/lore/main.go          ← CLI (Cobra), DI wiring, output formatting
internal/domain/          ← Memory struct, MemoryType enum, sentinel errors
internal/ports/           ← Interfaces only: StoragePort, AIPort, TimeParserPort, NotifierPort, ConfigPort
internal/app/             ← MemoryService, ReminderService (depend only on ports)
internal/adapters/
  sqlite/                 ← StoragePort + ConfigPort (pure-Go sqlite, WAL mode)
  ollama/                 ← AIPort (direct HTTP to Ollama REST, no SDK)
  timeparser/             ← TimeParserPort (wraps olebedev/when)
  notifier/               ← NotifierPort: notify-send with stdout fallback
  rcfile/                 ← ConfigPort from ~/.lorerc (key = value)
internal/testutil/mocks.go ← Mock implementations for all ports
```

**Dependency injection** happens in `main.go`'s `PersistentPreRunE`. Config priority: CLI flags > `~/.lorerc` > built-in defaults.

## Key Behaviors

- **Ollama is optional**: `MemoryService.Add()` runs Embed/DetectType/ExtractTags concurrently via `errgroup`, but gracefully degrades if Ollama is unavailable — the memory saves without enrichment.
- **Vector search is in-process**: `FindSimilar()` loads all embeddings and computes cosine similarity in Go. Swappable via `StoragePort`.
- **ULID prefix matching**: `GetByID` accepts 10-char prefixes of the full 26-char ULID.
- **Reminder daemon**: `lore daemon` runs a poll loop intended for use as a systemd user service.
- **Two config backends**: `rcfile` adapter reads `~/.lorerc`; `sqlite` adapter stores config in the DB. The CLI uses rcfile; the DB config table is reserved for future use.
