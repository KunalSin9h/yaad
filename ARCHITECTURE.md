# Architecture

`yaad` follows the **Ports and Adapters** (Hexagonal) pattern. Domain and application logic are fully isolated from infrastructure — every adapter is replaceable without touching business logic.

```
CLI (Cobra)
    │
Application Layer
    ├── MemoryService   — add, ask, list, delete
    └── ReminderService — check, daemon
         │
    Ports (interfaces)
    ├── StoragePort   ← SQLiteAdapter (modernc.org/sqlite, pure Go)
    ├── AIPort        ← OllamaAdapter (direct HTTP, no SDK dep)
    ├── TimeParserPort← WhenAdapter   (github.com/olebedev/when)
    ├── NotifierPort  ← NotifySend / Stdout (auto-detected)
    └── ConfigPort    ← RcfileAdapter (~/.yaadrc)
```

Swapping any layer requires implementing one interface. For example, to use ChromaDB for vector search, write a `ChromaAdapter` that satisfies `StoragePort` — the rest of the app is unchanged.

---

## Retrieval pipeline (`yaad ask`)

```
question
   │
   ▼  HyDE (ExpandQuery)
   │  LLM generates a hypothetical answer → embed that instead of the raw question
   │  → better semantic match to stored memories (10–30% recall improvement)
   │
   ▼  FindHybrid (BM25 + vector, RRF fusion)
   │  ├── BM25 leg  — SQLite FTS5 keyword search, returns ranked list
   │  └── Vector leg — cosine similarity over all stored embeddings
   │       ↓
   │  Reciprocal Rank Fusion  score = 1/(60+rank_bm25) + 1/(60+rank_vec)
   │  → single merged ranking, top 10 candidates
   │
   ▼  Rerank (optional, Qwen3-Reranker on Ollama)
   │  Cross-encoder scores each (query, candidate) pair directly
   │  → reorders by true contextual relevance, top 5 selected
   │
   ▼  Answer (LLM)
      Synthesises final answer from the top 5 memories
```

---

## Knowledge graph (`yaad add`)

Every memory addition asynchronously extracts named entities (people, projects, tools, concepts, places) and stores them in a SQLite graph (`entities` + `memory_entities`). This enables entity-centric retrieval — finding all memories that mention a specific person or project — independent of semantic similarity or exact wording.

---

## Project structure

```
yaad/
├── cmd/yaad/main.go          # entry point + dependency wiring
├── internal/
│   ├── domain/               # Memory, Entity, MemoryType, errors — no deps
│   ├── ports/                # interfaces only — no deps
│   ├── app/                  # business logic — depends only on ports
│   └── adapters/
│       ├── sqlite/           # StoragePort (hybrid search, entity graph, FTS5)
│       ├── ollama/           # AIPort (embed, chat, HyDE, rerank, entities)
│       ├── timeparser/       # TimeParserPort
│       ├── notifier/         # NotifierPort (notify-send + CLI)
│       └── rcfile/           # ConfigPort (~/.yaadrc)
├── SPEC.md                   # full product specification
├── ARCHITECTURE.md           # this file
└── CONFIG.md                 # configuration reference
```

---

## Adding new adapters

| Goal | Implement |
|---|---|
| New AI backend (OpenAI, Gemini) | `ports.AIPort` |
| New storage backend (ChromaDB, Postgres) | `ports.StoragePort` |
| New notifier (macOS, Slack, email) | `ports.NotifierPort` |

See `internal/testutil/mocks.go` for reference implementations of every port.
