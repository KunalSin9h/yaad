## yaad

> AI-native memory, recall and reminder on the terminal — locally with Ollama.
>
> **[yaad.knl.co.in](https://yaad.knl.co.in)**

Save anything from your terminal — commands, notes, URLs, facts, reminders — and recall it later with natural language. Everything runs locally. No cloud, no accounts.

```bash
# Save with optional context
yaad add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "yaad CLI build session"

# Set a time-based reminder
yaad add "book conference ticket" --remind "in 30 minutes"

# Ask anything — even indirectly
yaad ask "where was I with the yaad work?"
yaad ask "do I have anything due tonight?"
```

---

## Features

- **Hybrid retrieval** — BM25 full-text + semantic vector search merged via Reciprocal Rank Fusion
- **HyDE query expansion** — embeds a hypothetical answer rather than the raw question for better recall
- **Cross-encoder reranking** — optional Qwen3-Reranker re-scores results for true relevance
- **Entity knowledge graph** — extracts people, projects, tools from every memory; query by entity name
- **Smart reminders** — parse `"in 30 minutes"`, `"tomorrow 9am"`, `"Friday 3pm"` into real deadlines
- **Fully local** — all AI runs via [Ollama](https://ollama.com), no data leaves your machine
- **Offline-safe** — saves gracefully even when Ollama is not running

---

## Requirements

- [Go](https://go.dev) 1.21+
- [Ollama](https://ollama.com) running locally

```bash
ollama pull nomic-embed-text          # embeddings
ollama pull llama3.2:3b               # reasoning

# optional: cross-encoder reranking
ollama pull dengcao/Qwen3-Reranker-0.6B
```

---

## Installation

```bash
go install github.com/kunalsin9h/yaad/cmd/yaad@latest
```

Or build from source:

```bash
git clone https://github.com/kunalsin9h/yaad
cd yaad && make install
```

---

## Configuration

```bash
yaad config init                                                  # create ~/.yaadrc
yaad config set ollama.rerank_model dengcao/Qwen3-Reranker-0.6B  # optional
yaad config list
```

See [CONFIG.md](./CONFIG.md) for all keys, CLI flag overrides, and notifier options.

---

## Usage

```bash
# Save
yaad add "staging postgres is on port 5433" --for "backend infra" --tag postgres
yaad add "https://pkg.go.dev/modernc.org/sqlite" --for "pure Go SQLite, no CGO"

# Ask
yaad ask "what was the staging db port?"
yaad ask "everything I saved about payments-api"

# Browse
yaad list                    # 20 most recent
yaad list --type command
yaad list --tag postgres
yaad list --remind           # pending reminders only

# Lookup / delete
yaad get 01KKXKKJ3Q          # full details by ID prefix
yaad delete 01KKXKKJ3Q -y
yaad clean                   # delete all
```

---

## Reminders

Add `yaad check` to your shell prompt for inline reminders — no background process needed:

```bash
# ~/.zshrc
precmd() { yaad check }

# ~/.bashrc
export PROMPT_COMMAND="yaad check; $PROMPT_COMMAND"
```

Or run as a systemd user service with desktop notifications. See [REMINDERS.md](./REMINDERS.md) for full setup.

---

## Architecture

`yaad` follows the Ports and Adapters (Hexagonal) pattern. `yaad ask` runs a 4-stage retrieval pipeline: HyDE query expansion → hybrid BM25/vector search → cross-encoder reranking → LLM answer synthesis.

See [ARCHITECTURE.md](./ARCHITECTURE.md) for the full diagram, knowledge graph design, and adapter guide.

---

## License

MIT © [Kunal Singh](https://github.com/kunalsin9h)
