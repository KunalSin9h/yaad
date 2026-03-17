# lore — Product Specification

## Overview

`lore` is a terminal-native, AI-powered memory and reminder CLI. It lets you save any piece of information — commands, notes, URLs, facts, reminders — with rich metadata, then retrieve it later through natural language queries. The AI layer runs entirely locally via Ollama, keeping all data private and offline-capable.

It is **not** a calendar replacement. It is a queryable, intelligent scratchpad that lives in your terminal.

---

## Core Concepts

### Memory

A **Memory** is the atomic unit of the system. Every `add` creates one.

```
Memory {
  id          ULID            // sortable, unique
  content     string          // the actual thing you want to remember
  for         string          // human context: "why did I save this?"
  type        MemoryType      // auto-detected by AI
  tags        []string        // AI-extracted keywords
  working_dir string          // cwd at time of save
  hostname    string          // machine identity
  created_at  time.Time
  remind_at   *time.Time      // nil if not a reminder
  reminded_at *time.Time      // nil until notification fired
  embedding   []float32       // vector for semantic search
}
```

### Memory Types

| Type | Example |
|---|---|
| `command` | `claude --resume 17a43487-...` |
| `note` | `postgres runs on port 5433 in staging` |
| `reminder` | `book conference ticket before 8 PM` |
| `url` | `https://pkg.go.dev/...` |
| `fact` | `AWS account ID for prod is 123456789` |

Type is auto-detected by the AI on `add`. User can override with `--type`.

---

## CLI Interface

### `lore add`

```bash
lore add "<content>" [flags]

Flags:
  --for, -f   string   Human context / purpose label
  --remind    string   Natural language time: "in 30 minutes", "tomorrow 9am"
  --type      string   Override AI type detection
  --tag       string   Additional tag (repeatable)
```

Examples:
```bash
lore add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "rememberit CLI build session"

lore add "book conference ticket" \
  --remind "in 30 minutes"

lore add "postgres password is hunter2" \
  --for "staging env" --tag secrets
```

### `lore ask`

Natural language query. AI finds relevant memories and synthesizes an answer.

```bash
lore ask "<question>"
```

Examples:
```bash
lore ask "which claude session was I building rememberit in?"
lore ask "what do I need to do tonight?"
lore ask "what was that postgres port number?"
```

### `lore list`

```bash
lore list [flags]

Flags:
  --type    string   Filter by memory type
  --tag     string   Filter by tag
  --limit   int      Max results (default: 20)
  --remind         Show only pending reminders
```

### `lore get`

Retrieve a single memory by ID or fuzzy content match.

```bash
lore get <id>
lore get --like "claude resume"
```

### `lore delete`

```bash
lore delete <id>
```

### `lore daemon`

Background process that fires reminder notifications.

```bash
lore daemon start    # start in background
lore daemon stop
lore daemon status
lore daemon install  # install as systemd user service
```

### `lore check`

Designed to be called from shell `PROMPT_COMMAND`. Silently checks for due reminders and prints inline if any are found. Zero-latency alternative to daemon.

```bash
lore check           # prints nothing if no reminders due
```

Shell integration (add to `.bashrc` / `.zshrc`):
```bash
PROMPT_COMMAND="lore check; $PROMPT_COMMAND"
```

### `lore config`

```bash
lore config set ollama.url http://localhost:11434
lore config set ollama.embed_model nomic-embed-text
lore config set ollama.chat_model llama3.2:3b
lore config get ollama.url
lore config list
```

---

## Architecture — Ports and Adapters (Hexagonal)

The domain is isolated from all infrastructure. Ports are Go interfaces. Adapters are implementations that can be swapped.

```
┌─────────────────────────────────────────────────────┐
│                    CLI (Cobra)                       │  ← driving side
└────────────────────────┬────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────┐
│                   Application Layer                  │
│         MemoryService  │  ReminderService            │
└──────┬─────────────────┼──────────────────┬──────────┘
       │                 │                  │
  ┌────▼────┐      ┌─────▼─────┐     ┌─────▼──────┐
  │ Storage │      │    AI     │     │  Notifier  │   ← ports (interfaces)
  │  Port   │      │   Port    │     │   Port     │
  └────┬────┘      └─────┬─────┘     └─────┬──────┘
       │                 │                  │
  ┌────▼────┐      ┌─────▼─────┐     ┌─────▼──────┐
  │ SQLite  │      │  Ollama   │     │  notify-   │   ← adapters (implementations)
  │Adapter  │      │  Adapter  │     │send/plyer  │
  └─────────┘      └───────────┘     └────────────┘
```

### Ports (Interfaces)

```go
// StoragePort — all persistence operations
type StoragePort interface {
    Save(ctx context.Context, m *Memory) error
    GetByID(ctx context.Context, id string) (*Memory, error)
    List(ctx context.Context, filter ListFilter) ([]*Memory, error)
    Delete(ctx context.Context, id string) error
    FindSimilar(ctx context.Context, embedding []float32, topK int) ([]*Memory, error)
    PendingReminders(ctx context.Context, before time.Time) ([]*Memory, error)
    MarkReminded(ctx context.Context, id string) error
}

// AIPort — all intelligence operations
type AIPort interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    DetectType(ctx context.Context, content string) (MemoryType, error)
    ExtractTags(ctx context.Context, content, forLabel string) ([]string, error)
    Answer(ctx context.Context, question string, memories []*Memory) (string, error)
}

// TimeParserPort — natural language → time.Time
type TimeParserPort interface {
    Parse(expr string, from time.Time) (*time.Time, error)
}

// NotifierPort — delivery of reminder alerts
type NotifierPort interface {
    Notify(ctx context.Context, m *Memory) error
}

// ConfigPort — read/write app config
type ConfigPort interface {
    Get(key string) (string, error)
    Set(key, value string) error
    All() (map[string]string, error)
}
```

### Application Services

```go
// MemoryService — core business logic
type MemoryService struct {
    store   StoragePort
    ai      AIPort
    timer   TimeParserPort
}

func (s *MemoryService) Add(ctx, content, forLabel, remindExpr string, ...) (*Memory, error)
func (s *MemoryService) Ask(ctx, question string) (string, error)
func (s *MemoryService) List(ctx, filter ListFilter) ([]*Memory, error)
func (s *MemoryService) Delete(ctx, id string) error

// ReminderService — daemon / check logic
type ReminderService struct {
    store    StoragePort
    notifier NotifierPort
}

func (s *ReminderService) CheckAndFire(ctx context.Context) error
```

---

## Data Model (SQLite)

```sql
CREATE TABLE memories (
    id          TEXT PRIMARY KEY,
    content     TEXT NOT NULL,
    for_label   TEXT,
    type        TEXT NOT NULL DEFAULT 'note',
    tags        TEXT,              -- JSON array
    working_dir TEXT,
    hostname    TEXT,
    created_at  DATETIME NOT NULL,
    remind_at   DATETIME,
    reminded_at DATETIME,
    embedding   BLOB               -- float32 array, gob-encoded
);

CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX idx_memories_type       ON memories(type);
CREATE INDEX idx_memories_remind_at  ON memories(remind_at) WHERE remind_at IS NOT NULL;
CREATE INDEX idx_memories_created_at ON memories(created_at DESC);
```

---

## AI Strategy

### On `add`
1. Call `AIPort.Embed(content + " " + forLabel)` — store embedding
2. Call `AIPort.DetectType(content)` — unless `--type` overridden
3. Call `AIPort.ExtractTags(content, forLabel)` — enrich metadata
4. All three calls can be parallelized (goroutines)

### On `ask`
1. Embed the question
2. `StoragePort.FindSimilar(embedding, topK=5)` — vector recall
3. `AIPort.Answer(question, memories)` — LLM synthesizes final answer

### Models (defaults, all configurable)
| Purpose | Default Model |
|---|---|
| Embeddings | `nomic-embed-text` |
| Chat / reasoning | `llama3.2:3b` |

---

## Reminder Strategy

### Time Parsing
- `TimeParserPort` default adapter: `github.com/olebedev/when`
- Handles: `"in 30 minutes"`, `"tomorrow 9am"`, `"Friday 3pm"`, `"in 2 hours"`
- LLM is **not** used for time parsing — keep it deterministic and fast

### Notification Delivery
- Primary: `notify-send` (Linux desktop notification)
- Fallback: print to stdout (visible on next shell prompt via `check` command)
- Future: macOS `osascript`, Windows toast

### Reminder Trigger Window
- Fire when `remind_at <= now + 0s` and `reminded_at IS NULL`
- Daemon poll interval: 30 seconds
- `check` command: runs inline on every shell prompt (no background process needed)

---

## Configuration

Stored at `~/.config/lore/config.db` (same SQLite file).
Data stored at `~/.local/share/lore/memories.db`.

| Key | Default |
|---|---|
| `ollama.url` | `http://localhost:11434` |
| `ollama.embed_model` | `nomic-embed-text` |
| `ollama.chat_model` | `llama3.2:3b` |
| `notify.method` | `auto` (detect at runtime) |
| `reminder.poll_interval` | `30s` |
| `ui.time_format` | `relative` (e.g. "3 minutes ago") |

---

## Project Structure

```
rememberit/
├── cmd/
│   └── rememberit/
│       └── main.go                  # entry point, wire everything
├── internal/
│   ├── domain/
│   │   ├── memory.go                # Memory struct, MemoryType, ListFilter
│   │   └── errors.go                # domain errors
│   ├── ports/
│   │   ├── storage.go               # StoragePort interface
│   │   ├── ai.go                    # AIPort interface
│   │   ├── notifier.go              # NotifierPort interface
│   │   ├── timeparser.go            # TimeParserPort interface
│   │   └── config.go                # ConfigPort interface
│   ├── app/
│   │   ├── memory_service.go        # MemoryService
│   │   └── reminder_service.go      # ReminderService
│   └── adapters/
│       ├── sqlite/
│       │   └── store.go             # SQLiteAdapter → StoragePort
│       ├── ollama/
│       │   └── client.go            # OllamaAdapter → AIPort
│       ├── timeparser/
│       │   └── when.go              # WhenAdapter → TimeParserPort
│       └── notifier/
│           ├── notifysend.go        # Linux notify-send
│           └── stdout.go            # fallback stdout notifier
├── SPEC.md
├── PLAN.md
├── go.mod
└── go.sum
```

---

## Non-Goals

- Not a calendar (no recurring events, no invites)
- Not a task manager (no subtasks, projects, priorities)
- Not a sync service (local only, no cloud)
- Not a search engine (optimized for recall, not indexing everything)
- Not replacing `grep` through shell history

---

## Future Extensions (via new Adapters, zero core changes)

- `ChromaDBAdapter` → `StoragePort` for dedicated vector DB
- `OpenAIAdapter` → `AIPort` for cloud LLM fallback
- `SlackAdapter` → `NotifierPort` for Slack DMs
- `MacOSNotifierAdapter` → `NotifierPort` for macOS
- `GRPCServer` → driving port for programmatic access
