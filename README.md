<img width="1287" height="443" alt="image" src="https://github.com/user-attachments/assets/782e0ea7-143d-43e4-8e22-10c6b19a2740" />

## yaad

> Memory for your terminal.

[![Go](https://img.shields.io/badge/go-1.22+-blue)](https://go.dev) [![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE) [![yaad.knl.co.in](https://img.shields.io/badge/site-yaad.knl.co.in-purple)](https://yaad.knl.co.in/)

You forget the staging DB port. The curl flag that fixed that weird error. The deploy steps for this service. yaad saves it — and gives it back when you ask.

Runs entirely local via Ollama. No cloud, no accounts, no API keys.

```bash
$ yaad add "staging db is postgres on port 5433"
$ yaad add "deploy checklist: run migrations, restart workers, clear cache"

$ yaad ask "what port is staging on?"
→ Staging DB is postgres on port 5433.

$ yaad ask "what's the deploy checklist?"
→ Run migrations, restart workers, then clear cache.
```

---

## Install

**Linux / macOS:**

```bash
curl -fsSL https://yaad.knl.co.in/install.sh | bash
```

**Go install:**

```bash
go install github.com/kunalsin9h/yaad/cmd/yaad@latest
```

**Pre-built binaries** — [GitHub Releases](https://github.com/KunalSin9h/yaad/releases).

Then pull two Ollama models (one-time):

```bash
ollama pull mxbai-embed-large  # embeddings
ollama pull llama3.2:3b        # reasoning (or any chat model you prefer)
```

> Requires [Ollama](https://ollama.com) running locally.

---

## Reminders

yaad can remind you of things right in your terminal when they're due.

```bash
yaad add "submit PR for review" --remind "tomorrow 9am"
yaad add "book conference ticket" --remind "in 30 minutes"
```

See [REMINDERS.md](./docs/REMINDERS.md) for setup (shell hook or systemd daemon).

---

## Agent Integration

yaad ships as an [Agent Skill](https://github.com/vercel-labs/skills) — compatible with Claude Code, Cursor, Codex CLI, Gemini CLI, and any agent that supports the open skills standard.

```bash
npx skills add kunalsin9h/yaad -g
```

Once installed, your agent saves and recalls memory across sessions automatically.

---

## More

- [COMMANDS.md](./docs/COMMANDS.md) — full command reference (`list`, `get`, `delete`, `config`, all flags)
- [REMINDERS.md](./docs/REMINDERS.md) — reminder setup guide
- [CONFIG.md](./docs/CONFIG.md) — configuration keys, models, notifier options
- [SPEC.md](./docs/SPEC.md) — product specification and architecture

---

## License

MIT © [Kunal Singh](https://github.com/kunalsin9h)
