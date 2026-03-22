# yaad — Product Specification

## Overview

`yaad` is a terminal-native, AI-powered memory and reminder CLI. It lets you save any piece of information — commands, notes, URLs, facts, reminders — with rich metadata, then retrieve it later through natural language queries. The AI layer runs entirely locally via Ollama, keeping all data private and offline-capable.

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
  working_dir string          // cwd at time of save
  hostname    string          // machine identity
  created_at  time.Time
  remind_at   *time.Time      // nil if not a reminder
  reminded_at *time.Time      // nil until notification fired
  embedding   []float32       // vector for semantic search
}
```

---

## CLI Interface

### `yaad add`

```bash
yaad add "<content>" [flags]

Flags:
  --for, -f   string   Human context / purpose label
  --remind    string   Natural language time: "in 30 minutes", "tomorrow 9am"
```

Examples:
```bash
yaad add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "rememberit CLI build session"

yaad add "book conference ticket" \
  --remind "in 30 minutes"
```

### `yaad ask`

Natural language query. AI finds relevant memories and synthesizes an answer.

```bash
yaad ask "<question>"
```

Examples:
```bash
yaad ask "which claude session was I building rememberit in?"
yaad ask "what do I need to do tonight?"
yaad ask "what was that postgres port number?"
```

### `yaad list`

```bash
yaad list [flags]

Flags:
  --limit   int      Max results (default: 20)
  --remind         Show only pending reminders
```

### `yaad get`

Retrieve a single memory by ID or fuzzy content match.

```bash
yaad get <id>
yaad get --like "claude resume"
```

### `yaad delete`

```bash
yaad delete <id>
```

### `yaad daemon`

Background process that fires reminder notifications.

```bash
yaad daemon start    # start in background
yaad daemon stop
yaad daemon status
yaad daemon install  # install as systemd user service
```

### `yaad check`

Designed to be called from shell `PROMPT_COMMAND`. Silently checks for due reminders and prints inline if any are found. Zero-latency alternative to daemon.

```bash
yaad check           # prints nothing if no reminders due
```

Shell integration (add to `.bashrc` / `.zshrc`):
```bash
PROMPT_COMMAND="yaad check; $PROMPT_COMMAND"
```

### `yaad config`

```bash
yaad config set ollama.url http://localhost:11434
yaad config set ollama.embed_model mxbai-embed-large
yaad config set ollama.chat_model llama3.2:3b
yaad config get ollama.url
yaad config list
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

CREATE INDEX idx_memories_remind_at  ON memories(remind_at) WHERE remind_at IS NOT NULL;
CREATE INDEX idx_memories_created_at ON memories(created_at DESC);
```

---

## AI Strategy

### On `add`
1. Call `AIPort.Embed(content + " " + forLabel)` — store embedding

### On `ask`
1. Embed the question
2. `StoragePort.FindSimilar(embedding, topK=5)` — vector recall
3. `AIPort.Answer(question, memories)` — LLM synthesizes final answer

### Models (defaults, all configurable)
| Purpose | Default Model |
|---|---|
| Embeddings | `mxbai-embed-large` |
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

Stored at `~/.config/yaad/config.db` (same SQLite file).
Data stored at `~/.local/share/yaad/memories.db`.

| Key | Default |
|---|---|
| `ollama.url` | `http://localhost:11434` |
| `ollama.embed_model` | `mxbai-embed-large` |
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
│   │   ├── memory.go                # Memory struct, ListFilter
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
├── docs/
│   ├── SPEC.md
│   ├── COMMANDS.md
│   ├── REMINDERS.md
│   └── CONFIG.md
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
