<img width="1287" height="443" alt="image" src="https://github.com/user-attachments/assets/782e0ea7-143d-43e4-8e22-10c6b19a2740" />


AI-native memory, recall and reminder for you and your agent — locally with Ollama.

> [yaad website](https://yaad.knl.co.in/)

Save anything from your terminal — thoughts, notes, URLs, facts, reminders — and recall it later with natural language. Everything runs locally. No cloud, no accounts.

```bash
# Save a anything
yaad add "claude session for db migration: 3fa2c891-7b4e....."

# Set a time-based reminder
yaad add "book conference ticket" --remind "in 30 minutes"

# Ask anything
yaad ask "claude session for db migration?"
yaad ask "do I have anything due tonight?"
```

---

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Reminders](#reminders)
- [Architecture](#architecture)
- [Project structure](#project-structure)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **Query-first** — natural language search powered by local embeddings + LLM
- **Rich metadata** — every memory captures working directory, hostname, and timestamp automatically
- **Smart reminders** — parse `"in 30 minutes"`, `"tomorrow 9am"`, `"Friday 3pm"` into real deadlines
- **Terminal-native notifications** — reminder daemon via systemd, or inline via shell `PROMPT_COMMAND`
- **Fully local** — all AI runs via [Ollama](https://ollama.com), no data leaves your machine
- **Offline-safe** — saves gracefully even when Ollama is not running
- **Ports & Adapters architecture** — every component is swappable (storage, AI, notifier)

---

## Requirements

- [Ollama](https://ollama.com) running locally

Pull the required models once:

```bash
ollama pull mxbai-embed-large  # embeddings
ollama pull llama3.2:3b        # reasoning (or any chat model you prefer)
```

---

## Installation

**Linux / macOS** — one-liner (detects platform and arch automatically):

```bash
curl -fsSL https://raw.githubusercontent.com/KunalSin9h/yaad/main/scripts/install.sh | bash
```

**Go install:**

```bash
go install github.com/kunalsin9h/yaad/cmd/yaad@latest
```

**Pre-built binaries** — download from [GitHub Releases](https://github.com/KunalSin9h/yaad/releases).

**Build from source** (requires [Go](https://go.dev) 1.21+):

```bash
git clone https://github.com/kunalsin9h/yaad
cd yaad
make install
```

---

## Configuration

Configuration is read from `~/.yaadrc`. Generate it with commented defaults:

```bash
yaad config init
```

Common commands:

```bash
yaad config set ollama.chat_model mistral
yaad config list
```

See [CONFIG.md](./CONFIG.md) for all keys, notifier options, CLI flag overrides, and data storage location.

---

## Usage

### Save a memory

```bash
yaad add "<content>" [flags]

Flags:
      --remind  string   When to remind you ("in 30 minutes", "tomorrow 9am")
      --type    string   Override type detection: command|note|reminder|url|fact
      --tag     string   Add a tag (repeatable)
```

Examples:

```bash
# Just save something
yaad add "kubectl rollout restart deployment/api"

# Include context right in the content — AI will find it later
yaad add "yaad build session: claude --resume 17a43487-5ce9-4fd3-a9b5-b099d335f644"

# A time-sensitive reminder
yaad add "book conference ticket" --remind "in 30 minutes"

# A fact with tags
yaad add "backend infra — staging postgres is on port 5433" --tag postgres --tag staging

# A URL with context
yaad add "pure Go SQLite driver, no CGO: https://pkg.go.dev/modernc.org/sqlite"
```

### Query your memories

```bash
yaad ask "which claude session was for building yaad?"
yaad ask "what was the staging postgres port?"
yaad ask "do I have anything due tonight?"
```

### Browse memories

```bash
yaad list                   # 20 most recent
yaad list --type command    # only commands
yaad list --tag postgres    # by tag
yaad list --remind          # pending reminders only
yaad list --limit 50
```

### Get full details

```bash
yaad get 01KKXKKJ3Q         # by ID (prefix is fine)
```

Output includes content, context label, type, tags, working directory, hostname, and timestamps.

### Delete

```bash
yaad delete 01KKXKKJ3Q      # prompts for confirmation
yaad delete 01KKXKKJ3Q -y   # skip confirmation
```

---

## Reminders

### Inline — shell `PROMPT_COMMAND` (recommended)

Reminders surface directly in your terminal on every prompt — no background process needed.

Add to your `~/.bashrc` or `~/.zshrc`:

```bash
export PROMPT_COMMAND="yaad check; $PROMPT_COMMAND"
```

For `zsh`, add to `~/.zshrc`:

```zsh
precmd() { yaad check }
```

### Background daemon — systemd user service

```bash
yaad daemon install          # writes ~/.config/systemd/user/yaad.service
systemctl --user enable --now yaad
```

Check status:

```bash
systemctl --user status yaad
```

---

## Architecture

`yaad` follows the **Ports and Adapters** (Hexagonal) pattern. The domain and application logic are fully isolated from infrastructure — every adapter is replaceable without touching business logic.

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
yaad/
├── cmd/yaad/main.go          # entry point + dependency wiring
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
