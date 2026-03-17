rememberit

> I AI-native terminal memory and reminder system, powered by Ollama.

Save anything from your terminal — commands, notes, URLs, facts, reminders — and recall it later with natural language. Everything runs locally. No cloud, no accounts.

```bash
# Save a command with context
lore add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "rememberit CLI build session"

# Set a time-based reminder
lore add "book conference ticket" --remind "in 30 minutes"

# Ask anything
lore ask "which claude session was I building rememberit in?"
lore ask "do I have anything due tonight?"
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
go install github.com/kunalsin9h/lore/cmd/rememberit@latest
```

Or build from source:

```bash
git clone https://github.com/kunalsin9h/lore
cd rememberit
make install
```

---

## Usage

### Save a memory

```bash
lore add "<content>" [flags]

Flags:
  -f, --for     string   Why are you saving this? (context label)
      --remind  string   When to remind you ("in 30 minutes", "tomorrow 9am")
      --type    string   Override type detection: command|note|reminder|url|fact
      --tag     string   Add a tag (repeatable)
```

Examples:

```bash
# A command you want to resume later
lore add "claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644" \
  --for "rememberit build session"

# A time-sensitive reminder
lore add "book conference ticket" --remind "in 30 minutes"

# A fact with tags
lore add "staging postgres is on port 5433" \
  --for "backend infra" --tag postgres --tag staging

# A URL
lore add "https://pkg.go.dev/modernc.org/sqlite" \
  --for "pure Go SQLite driver, no CGO"
```

### Query your memories

```bash
lore ask "which claude session was for building rememberit?"
lore ask "what was the staging postgres port?"
lore ask "do I have anything due tonight?"
```

### Browse memories

```bash
lore list                   # 20 most recent
lore list --type command    # only commands
lore list --tag postgres    # by tag
lore list --remind          # pending reminders only
lore list --limit 50
```

### Get full details

```bash
lore get 01KKXKKJ3Q         # by ID (prefix is fine)
```

Output includes content, context label, type, tags, working directory, hostname, and timestamps.

### Delete

```bash
lore delete 01KKXKKJ3Q      # prompts for confirmation
lore delete 01KKXKKJ3Q -y   # skip confirmation
```

---

## Reminders

### Inline — shell `PROMPT_COMMAND` (recommended)

Reminders surface directly in your terminal on every prompt — no background process needed.

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
export PROMPT_COMMAND="lore check; $PROMPT_COMMAND"
```

For `zsh`, add to `~/.zshrc`:

```zsh
precmd() { lore check }
```

### Background daemon — systemd user service

```bash
lore daemon install          # writes ~/.config/systemd/user/lore.service
systemctl --user enable --now rememberit
```

Check status:

```bash
systemctl --user status rememberit
```

---

## Configuration

Configuration is read from `~/.lorerc`. Create it with defaults:

```bash
lore config init
```

This generates a commented file you can edit directly:

```ini
# lore configuration

# Ollama server
ollama.url           = http://localhost:11434

# Embedding model — nomic-embed-text, mxbai-embed-large, all-minilm
ollama.embed_model   = nomic-embed-text

# Chat model — llama3.2:3b, mistral, gemma2:2b, phi3
ollama.chat_model    = llama3.2:3b

# Reminder daemon poll interval
reminder.poll_interval = 30s
```

**Priority: built-in defaults < `~/.lorerc` < CLI flags**

CLI flags override the rc file for a single invocation:

```bash
rememberit --chat-model mistral ask "what was that command?"
rememberit --ollama-url http://192.168.1.5:11434 add "remote ollama note"
```

Config commands work on `~/.lorerc` directly:

```bash
lore config list                            # show all set values
lore config set ollama.chat_model mistral   # update a value
lore config get ollama.chat_model           # read a value
lore config path                            # print rc file location
```

| Key | Default |
|---|---|
| `ollama.url` | `http://localhost:11434` |
| `ollama.embed_model` | `nomic-embed-text` |
| `ollama.chat_model` | `llama3.2:3b` |
| `reminder.poll_interval` | `30s` |

Data is stored at `$XDG_DATA_HOME/lore/memories.db` (defaults to `~/.local/share/lore/memories.db`).

---

## Architecture

`lore` follows the **Ports and Adapters** (Hexagonal) pattern. The domain and application logic are fully isolated from infrastructure — every adapter is replaceable without touching business logic.

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
├── cmd/lore/main.go          # entry point + dependency wiring
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
