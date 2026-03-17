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
  id          ULID            // sortable, unique (26 chars; prefix-matched at 10)
  content     string          // the actual thing you want to remember
  for_label   string          // human context: "why did I save this?"
  type        MemoryType      // auto-detected by AI
  tags        []string        // AI-extracted keywords + user tags
  working_dir string          // cwd at time of save
  hostname    string          // machine identity
  created_at  time.Time
  remind_at   *time.Time      // nil if not a reminder
  reminded_at *time.Time      // nil until notification fired
  embedding   []float32       // vector for semantic search (gob-encoded BLOB)
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

### Entity

An **Entity** is a named thing extracted from memory content — a person, project, tool, concept, or place. Entities form the nodes of the knowledge graph and are linked to memories via a junction table.

```
Entity {
  id         ULID
  name       string      // normalised to lowercase
  type       EntityType  // person | place | project | concept | tool
  created_at time.Time
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
  --type      string   Override AI type detection
  --tag       string   Additional tag (repeatable)
```

Examples:
```bash
yaad add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "yaad CLI build session"

yaad add "book conference ticket" \
  --remind "in 30 minutes"

yaad add "postgres password is hunter2" \
  --for "staging env" --tag secrets
```

### `yaad ask`

Natural language query. Hybrid retrieval + optional reranking + LLM synthesis.

```bash
yaad ask "<question>"
```

Examples:
```bash
yaad ask "which claude session was I building yaad in?"
yaad ask "what do I need to do tonight?"
yaad ask "what was that postgres port number?"
```

### `yaad list`

```bash
yaad list [flags]

Flags:
  --type    string   Filter by memory type
  --tag     string   Filter by tag
  --limit   int      Max results (default: 20)
  --remind         Show only pending reminders
```

### `yaad get`

Retrieve a single memory by ID (full or 10-char prefix).

```bash
yaad get <id>
```

### `yaad delete`

```bash
yaad delete <id>         # prompts for confirmation
yaad delete <id> -y      # skip confirmation
```

### `yaad clean`

```bash
yaad clean               # delete ALL memories (with confirmation)
yaad clean -y
```

### `yaad daemon`

Background process that fires reminder notifications.

```bash
yaad daemon start    # start in foreground
yaad daemon install  # install as systemd user service
```

### `yaad check`

Designed to run from shell `PROMPT_COMMAND`. Silently checks for due reminders.

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
yaad config set ollama.embed_model nomic-embed-text
yaad config set ollama.chat_model llama3.2:3b
yaad config set ollama.rerank_model dengcao/Qwen3-Reranker-0.6B
yaad config get ollama.url
yaad config list
yaad config init   # generate ~/.yaadrc with commented defaults
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
  │Adapter  │      │  Adapter  │     │send / CLI  │
  └─────────┘      └───────────┘     └────────────┘
```

### Ports (Interfaces)

```go
// StoragePort — all persistence and retrieval operations
type StoragePort interface {
    Save(ctx context.Context, m *Memory) error
    GetByID(ctx context.Context, id string) (*Memory, error)
    List(ctx context.Context, filter ListFilter) ([]*Memory, error)
    Delete(ctx context.Context, id string) error
    DeleteAll(ctx context.Context) (int64, error)

    // FindSimilar: pure cosine similarity over all stored embeddings (fallback).
    FindSimilar(ctx context.Context, embedding []float32, topK int) ([]*Memory, error)

    // FindHybrid: BM25 + cosine merged via Reciprocal Rank Fusion (preferred).
    FindHybrid(ctx context.Context, query string, embedding []float32, topK int) ([]*Memory, error)

    // Knowledge graph.
    SaveEntities(ctx context.Context, memoryID string, entities []Entity) error
    FindByEntities(ctx context.Context, names []string, topK int) ([]*Memory, error)

    PendingReminders(ctx context.Context, before time.Time) ([]*Memory, error)
    MarkReminded(ctx context.Context, id string) error
}

// AIPort — all intelligence operations
type AIPort interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    DetectType(ctx context.Context, content string) (MemoryType, error)
    ExtractTags(ctx context.Context, content, forLabel string) ([]string, error)
    Answer(ctx context.Context, question string, memories []*Memory) (string, error)

    // HyDE: generate a hypothetical answer to embed instead of the question.
    ExpandQuery(ctx context.Context, question string) (string, error)

    // Cross-encoder reranking using Qwen3-Reranker (optional).
    Rerank(ctx context.Context, query string, candidates []*Memory) ([]*Memory, error)

    // Entity extraction for the knowledge graph.
    ExtractEntities(ctx context.Context, content string) ([]Entity, error)
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

func (s *MemoryService) Add(ctx, req AddRequest) (*Memory, error)
func (s *MemoryService) Ask(ctx, question string) (string, error)
func (s *MemoryService) List(ctx, filter ListFilter) ([]*Memory, error)
func (s *MemoryService) GetByID(ctx, id string) (*Memory, error)
func (s *MemoryService) Delete(ctx, id string) error
func (s *MemoryService) Clean(ctx) (int64, error)

// ReminderService — daemon / check logic
type ReminderService struct {
    store    StoragePort
    notifier NotifierPort
}

func (s *ReminderService) CheckAndFire(ctx context.Context) error
func (s *ReminderService) RunDaemon(ctx context.Context, interval time.Duration) error
```

---

## Retrieval Technology

This section documents the core retrieval pipeline powering `yaad ask`. The design
prioritises quality and correctness at personal scale (< 100k memories) without
external infrastructure dependencies.

### 1. Embedding Model

- **Default model**: `nomic-embed-text` (768 dimensions) via Ollama
- **Storage**: gob-encoded `[]float32` BLOB in SQLite
- **At add time**: content + for_label are concatenated and embedded together for richer context
- **Graceful degradation**: memories without embeddings are excluded from vector search but still appear in BM25 results

### 2. BM25 Full-Text Search (SQLite FTS5)

SQLite's native FTS5 module provides BM25 ranking with zero additional dependencies.

```sql
CREATE VIRTUAL TABLE memories_fts USING fts5(
    content,
    for_label,
    content='memories',   -- reads data from the memories table (no duplication)
    content_rowid='rowid'
);
```

- **Sync**: triggers on INSERT/UPDATE/DELETE keep the FTS index in sync automatically
- **Migration**: `INSERT INTO memories_fts(memories_fts) VALUES ('rebuild')` on startup rebuilds from existing data
- **When it wins**: exact keywords, command names, tool names, person names, hostnames

### 3. Reciprocal Rank Fusion (RRF)

After running both BM25 and cosine-similarity searches, results are merged using RRF:

```
score(doc) = 1/(60 + rank_bm25) + 1/(60 + rank_vector)
```

The constant `k=60` is from the original RRF paper (Cormack et al., 2009). It prevents very high ranks from dominating, making the fusion stable across different query types.

**Why RRF over score normalisation**: BM25 scores and cosine distances are on different scales and distributions. RRF only uses rank positions, so there is no calibration needed. Research shows 15–30% retrieval precision improvements over single-strategy search.

### 4. HyDE — Hypothetical Document Embeddings

Before embedding the user's question, `ExpandQuery()` asks the LLM to write a short hypothetical passage that would _answer_ the question. That passage is embedded instead of the raw question.

**Why it helps**: Embeddings are trained on (document, document) similarity, not (question, document). A hypothetical answer lives in the same embedding space as stored memories, dramatically improving recall on abstract or indirect queries.

**Graceful degradation**: if the LLM is unavailable, the original question text is embedded as before.

### 5. Cross-Encoder Reranking (Qwen3-Reranker)

After hybrid retrieval returns a pool of ~10 candidates, an optional cross-encoder re-scores each `(query, candidate)` pair:

- **Model**: `dengcao/Qwen3-Reranker-0.6B` (or `4B`, `8B`) via Ollama
- **How it works**: unlike bi-encoder embeddings that score query and document independently, a cross-encoder processes both together — enabling nuanced contextual relevance judgment
- **API**: uses `/api/chat` with a structured yes/no judgment prompt; candidates are sorted descending by "yes" confidence
- **Config**: `ollama.rerank_model` in `~/.yaadrc` (empty = disabled, which is the default)
- **Graceful degradation**: if the model is unavailable or not configured, the hybrid-ranked order is preserved

### 6. Entity Knowledge Graph

Entities (people, projects, tools, concepts, places) are extracted from every saved memory and stored in a SQLite graph:

```sql
-- Nodes
CREATE TABLE entities (
    id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL,
    created_at DATETIME NOT NULL, UNIQUE(name, type)
);

-- Edges (memory ↔ entity)
CREATE TABLE memory_entities (
    memory_id TEXT REFERENCES memories(id) ON DELETE CASCADE,
    entity_id TEXT REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (memory_id, entity_id)
);
```

- **Extraction**: async goroutine after `Save()` — never blocks the add
- **Retrieval**: `FindByEntities(names, topK)` — JOIN-based, ordered by recency
- **Use case**: "show me everything about Alice" or "memories related to payments-api" regardless of exact wording

### Full `ask` pipeline summary

```
question
   │
   ▼ ExpandQuery (HyDE)
   │  LLM generates hypothetical answer → embed that
   │
   ▼ FindHybrid (topK=10)
   │  BM25 (FTS5)  ──┐
   │  cosine sim  ───┤ RRF fusion
   │                 ↓
   │  merged ranked list
   │
   ▼ Rerank (optional, Qwen3-Reranker)
   │  cross-encoder rescores → top 5
   │
   ▼ Answer (LLM)
     synthesised response
```

---

## Data Model (SQLite)

```sql
-- Core memory storage
CREATE TABLE memories (
    id          TEXT PRIMARY KEY,
    content     TEXT NOT NULL,
    for_label   TEXT NOT NULL DEFAULT '',
    type        TEXT NOT NULL DEFAULT 'note',
    tags        TEXT NOT NULL DEFAULT '[]',  -- JSON array
    working_dir TEXT NOT NULL DEFAULT '',
    hostname    TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL,
    remind_at   DATETIME,
    reminded_at DATETIME,
    embedding   BLOB          -- []float32, gob-encoded
);

CREATE INDEX idx_memories_type       ON memories(type);
CREATE INDEX idx_memories_remind_at  ON memories(remind_at) WHERE remind_at IS NOT NULL;
CREATE INDEX idx_memories_created_at ON memories(created_at DESC);

-- FTS5 index (BM25 keyword search)
CREATE VIRTUAL TABLE memories_fts USING fts5(
    content, for_label,
    content='memories', content_rowid='rowid'
);

-- Sync triggers omitted for brevity; see adapters/sqlite/db.go

-- Knowledge graph
CREATE TABLE entities (
    id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL,
    created_at DATETIME NOT NULL, UNIQUE(name, type)
);

CREATE TABLE memory_entities (
    memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (memory_id, entity_id)
);

-- App configuration
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

---

## AI Strategy

### On `add`

1. Concurrently (via `errgroup`):
   - `Embed(content + " " + for_label)` → store embedding
   - `DetectType(content)` → unless `--type` overridden
   - `ExtractTags(content, forLabel)` → enrich metadata
2. After `Save()` completes (async goroutine):
   - `ExtractEntities(content)` → `SaveEntities(memoryID, entities)`
3. All AI errors are non-fatal — memory is saved even if Ollama is unreachable

### On `ask`

1. `ExpandQuery(question)` → hypothetical passage (HyDE)
2. `Embed(expandedQuery)` → query vector
3. `FindHybrid(question, vector, topK=10)` → BM25 + cosine + RRF
4. `Rerank(question, candidates)` → cross-encoder rescore (if configured)
5. `Answer(question, top5)` → LLM response

### Models (defaults, all configurable via `~/.yaadrc`)

| Purpose | Default | Config key |
|---|---|---|
| Embeddings | `nomic-embed-text` | `ollama.embed_model` |
| Chat / reasoning | `llama3.2:3b` | `ollama.chat_model` |
| Reranking | _(empty — disabled)_ | `ollama.rerank_model` |

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

### Trigger Window

- Fire when `remind_at <= now` and `reminded_at IS NULL`
- Daemon poll interval: 30 seconds (configurable via `reminder.poll_interval`)
- `check` runs inline on every shell prompt — no background process needed

---

## Configuration

Config file: `~/.yaadrc` (key = value). Generate with `yaad config init`.
Data directory: `~/.local/share/yaad/` (respects `$XDG_DATA_HOME`).

| Key | Default | Description |
|---|---|---|
| `ollama.url` | `http://localhost:11434` | Ollama server URL |
| `ollama.embed_model` | `nomic-embed-text` | Embedding model |
| `ollama.chat_model` | `llama3.2:3b` | Chat/reasoning model |
| `ollama.rerank_model` | _(empty)_ | Reranker model; empty disables reranking |
| `notifier` | `cli` | Notifier: `cli`, `notify-send`, or comma-separated |
| `reminder.poll_interval` | `30s` | Daemon polling interval |

CLI flags override rc file; rc file overrides built-in defaults.

---

## Project Structure

```
yaad/
├── cmd/yaad/main.go          # entry point + dependency wiring (DI in PersistentPreRunE)
├── internal/
│   ├── domain/
│   │   ├── memory.go         # Memory struct, MemoryType, ListFilter
│   │   ├── entity.go         # Entity struct, EntityType
│   │   └── errors.go         # sentinel errors
│   ├── ports/
│   │   ├── storage.go        # StoragePort interface
│   │   ├── ai.go             # AIPort interface
│   │   ├── notifier.go       # NotifierPort interface
│   │   ├── timeparser.go     # TimeParserPort interface
│   │   └── config.go         # ConfigPort interface
│   ├── app/
│   │   ├── memory_service.go    # MemoryService (add, ask, list, delete, clean)
│   │   └── reminder_service.go  # ReminderService (check, daemon)
│   ├── adapters/
│   │   ├── sqlite/
│   │   │   ├── db.go         # schema, Open(), migrations, FTS5 triggers
│   │   │   ├── store.go      # StoragePort impl (hybrid search, entity graph)
│   │   │   └── config.go     # ConfigPort impl
│   │   ├── ollama/
│   │   │   └── client.go     # AIPort impl (embed, detect, tags, answer, rerank, HyDE, entities)
│   │   ├── timeparser/
│   │   │   └── when.go       # TimeParserPort impl
│   │   ├── notifier/
│   │   │   ├── cli.go        # stdout notifier
│   │   │   ├── notifysend.go # notify-send notifier
│   │   │   └── multi.go      # fan-out notifier
│   │   └── rcfile/
│   │       └── config.go     # ConfigPort from ~/.yaadrc
│   └── testutil/
│       └── mocks.go          # mock implementations of all ports
├── scripts/
│   └── test_cli.sh           # end-to-end CLI integration tests
├── SPEC.md                   # this file
├── CONFIG.md                 # configuration reference
├── CLAUDE.md                 # guidance for Claude Code
└── Makefile
```

---

## Non-Goals

- Not a calendar (no recurring events, no invites)
- Not a task manager (no subtasks, projects, priorities)
- Not a sync service (local only, no cloud)
- Not replacing `grep` through shell history

---

## Future Extensions (via new Adapters, zero core changes)

- `ChromaDBAdapter` → `StoragePort` for dedicated vector DB at larger scale
- `HNSWAdapter` — in-memory HNSW index (`github.com/coder/hnsw`) for O(log n) ANN search instead of O(n) brute-force cosine; worthwhile when embeddings exceed ~500k
- `OpenAIAdapter` → `AIPort` for cloud LLM fallback
- `SlackAdapter` → `NotifierPort` for Slack DMs
- `MacOSNotifierAdapter` → `NotifierPort` for macOS
- `GRPCServer` → driving port for programmatic / agent access
- `MCPServer` → expose yaad memories as a Model Context Protocol tool for Claude
