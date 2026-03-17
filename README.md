<p align="center">
  <img src="logo.svg" width="96" height="96" alt="rememberit logo"/>
</p>

<h1 align="center">rememberit</h1>

<p align="center">AI-native terminal memory and reminder system, powered by Ollama.</p>

Save anything from your terminal — commands, notes, URLs, facts, reminders — and recall it later with natural language. Everything runs locally. No cloud, no accounts.

```bash
# Save a command with context
rememberit add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "rememberit CLI build session"

# Set a time-based reminder
rememberit add "book conference ticket" --remind "in 30 minutes"

# Ask anything
rememberit ask "which claude session was I building rememberit in?"
rememberit ask "do I have anything due tonight?"
```

---

## Features

- **Query-first** — natural language search powered by local embeddings + LLM
- **Rich metadata** — every memory captures `--for` context, working directory, hostname, and timestamp automatically
- **Smart reminders** — parse `"in 30 minutes"`, `"tomorrow 9am"`, `"Friday 3pm"` into real deadlines
- **Terminal-native notifications** — reminder daemon via systemd, or inline via shell `PROMPT_COMMAND`
- **Fully local** — all AI runs via [Ollama](https://ollama.com), no data leaves your machine
- **Offline-safe** — saves gracefully even when Ollama is not running
- **Ports & Adapters architecture** — every component is swappable (storage, AI, notifier)

---

## Requirements

- [Go](https://go.dev) 1.21+
- [Ollama](https://ollama.com) running locally

Pull the required models once:

```bash
ollama pull nomic-embed-text   # embeddings
ollama pull llama3.2:3b        # reasoning (or any chat model you prefer)
```

---

## Installation

```bash
go install github.com/kunalsin9h/rememberit/cmd/rememberit@latest
```

Or build from source:

```bash
git clone https://github.com/kunalsin9h/rememberit
cd rememberit
make install
```

---

## Usage

### Save a memory

```bash
rememberit add "<content>" [flags]

Flags:
  -f, --for     string   Why are you saving this? (context label)
      --remind  string   When to remind you ("in 30 minutes", "tomorrow 9am")
      --type    string   Override type detection: command|note|reminder|url|fact
      --tag     string   Add a tag (repeatable)
```

Examples:

```bash
# A command you want to resume later
rememberit add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "rememberit build session"

# A time-sensitive reminder
rememberit add "book conference ticket" --remind "in 30 minutes"

# A fact with tags
rememberit add "staging postgres is on port 5433" \
  --for "backend infra" --tag postgres --tag staging

# A URL
rememberit add "https://pkg.go.dev/modernc.org/sqlite" \
  --for "pure Go SQLite driver, no CGO"
```

### Query your memories

```bash
rememberit ask "which claude session was for building rememberit?"
rememberit ask "what was the staging postgres port?"
rememberit ask "do I have anything due tonight?"
```

### Browse memories

```bash
rememberit list                   # 20 most recent
rememberit list --type command    # only commands
rememberit list --tag postgres    # by tag
rememberit list --remind          # pending reminders only
rememberit list --limit 50
```

### Get full details

```bash
rememberit get 01KKXKKJ3Q         # by ID (prefix is fine)
```

Output includes content, context label, type, tags, working directory, hostname, and timestamps.

### Delete

```bash
rememberit delete 01KKXKKJ3Q      # prompts for confirmation
rememberit delete 01KKXKKJ3Q -y   # skip confirmation
```

---

## Reminders

### Inline — shell `PROMPT_COMMAND` (recommended)

Reminders surface directly in your terminal on every prompt — no background process needed.

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
export PROMPT_COMMAND="rememberit check; $PROMPT_COMMAND"
```

For `zsh`, add to `~/.zshrc`:

```zsh
precmd() { rememberit check }
```

### Background daemon — systemd user service

```bash
rememberit daemon install          # writes ~/.config/systemd/user/rememberit.service
systemctl --user enable --now rememberit
```

Check status:

```bash
systemctl --user status rememberit
```

---

## Configuration

```bash
rememberit config list                                   # show all values
rememberit config set ollama.url http://localhost:11434
rememberit config set ollama.embed_model nomic-embed-text
rememberit config set ollama.chat_model llama3.2:3b
rememberit config set reminder.poll_interval 30s
```

| Key | Default |
|---|---|
| `ollama.url` | `http://localhost:11434` |
| `ollama.embed_model` | `nomic-embed-text` |
| `ollama.chat_model` | `llama3.2:3b` |
| `reminder.poll_interval` | `30s` |

Data is stored at `$XDG_DATA_HOME/rememberit/memories.db` (defaults to `~/.local/share/rememberit/memories.db`).

---

## Architecture

`rememberit` follows the **Ports and Adapters** (Hexagonal) pattern. The domain and application logic are fully isolated from infrastructure — every adapter is replaceable without touching business logic.

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
    └── ConfigPort    ← SQLiteAdapter (same DB, config table)
```

Swapping any layer requires implementing one interface. For example, to use ChromaDB for vector search, write a `ChromaAdapter` that satisfies `StoragePort` — the rest of the app is unchanged.

---

## Project structure

```
rememberit/
├── cmd/rememberit/main.go          # entry point + dependency wiring
├── internal/
│   ├── domain/                     # Memory, MemoryType, errors — no deps
│   ├── ports/                      # interfaces only — no deps
│   ├── app/                        # business logic — depends only on ports
│   └── adapters/
│       ├── sqlite/                 # StoragePort + ConfigPort
│       ├── ollama/                 # AIPort (direct HTTP)
│       ├── timeparser/             # TimeParserPort
│       └── notifier/               # NotifierPort (notify-send + stdout)
├── SPEC.md                         # product specification
├── PLAN.md                         # implementation checklist
└── Makefile
```

---

## Contributing

Contributions are welcome. The architecture is designed to make new adapters easy to add:

- **New AI backend** (OpenAI, Gemini) → implement `ports.AIPort`
- **New storage backend** (ChromaDB, Postgres) → implement `ports.StoragePort`
- **New notifier** (macOS, Slack, email) → implement `ports.NotifierPort`

Please open an issue before starting large changes.

---

## License

MIT © [Kunal Singh](https://github.com/kunalsin9h)
