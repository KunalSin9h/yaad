# Configuration

Configuration is read from `~/.yaadrc` (key = value format). Create it with commented defaults:

```bash
yaad config init
```

---

## Config file (`~/.yaadrc`)

```ini
# yaad configuration

# Ollama server
ollama.url           = http://localhost:11434

# Embedding model — nomic-embed-text, mxbai-embed-large, all-minilm
ollama.embed_model   = nomic-embed-text

# Chat model — llama3.2:3b, mistral, gemma2:2b, phi3
ollama.chat_model    = llama3.2:3b

# Rerank model — cross-encoder for hybrid retrieval reranking (optional)
# Requires: ollama pull dengcao/Qwen3-Reranker-0.6B
# Leave unset to skip reranking.
ollama.rerank_model  = dengcao/Qwen3-Reranker-0.6B

# Reminder daemon poll interval
reminder.poll_interval = 30s

# Notifier adapters — comma-separated, all fire together
# cli         — styled box printed to terminal (default, no dependencies)
# notify-send — desktop notification via notify-send (Linux only)
notifier = cli
```

---

## All keys

| Key | Default | Description |
|---|---|---|
| `ollama.url` | `http://localhost:11434` | Ollama server URL |
| `ollama.embed_model` | `nomic-embed-text` | Model for generating embeddings |
| `ollama.chat_model` | `llama3.2:3b` | Model for query answering and type detection |
| `ollama.rerank_model` | _(unset)_ | Cross-encoder rerank model; empty = skip reranking. Requires `ollama pull dengcao/Qwen3-Reranker-0.6B` |
| `reminder.poll_interval` | `30s` | How often the daemon polls for due reminders |
| `notifier` | `cli` | Notifier adapter(s), comma-separated |

---

## Priority

```
built-in defaults  <  ~/.yaadrc  <  CLI flags
```

CLI flags override the rc file for a single invocation:

```bash
yaad --chat-model mistral ask "what was that command?"
yaad --ollama-url http://192.168.1.5:11434 add "remote ollama note"
yaad --rerank-model dengcao/Qwen3-Reranker-0.6B ask "who did I meet last week?"
yaad --notifier cli,notify-send check
```

---

## Config commands

```bash
yaad config init                            # create ~/.yaadrc with defaults
yaad config list                            # show all set values
yaad config set ollama.chat_model mistral   # update a value
yaad config get ollama.chat_model           # read a value
yaad config path                            # print rc file location
```

---

## Data storage

Memories are stored at `$XDG_DATA_HOME/yaad/memories.db`, defaulting to `~/.local/share/yaad/memories.db`.

---

## Notifiers

Notifiers are composable — set multiple to fire all at once:

```bash
yaad config set notifier cli,notify-send
```

| Adapter | Platform | Requirement |
|---|---|---|
| `cli` | All | None — prints a styled box to the terminal |
| `notify-send` | Linux | `notify-send` must be installed (`libnotify-bin`) |
