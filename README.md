<img width="1287" height="443" alt="image" src="https://github.com/user-attachments/assets/782e0ea7-143d-43e4-8e22-10c6b19a2740" />

## yaad

> The simplest local memory engine — for you and your agents.

> [yaad.knl.co.in](https://yaad.knl.co.in/)

No servers. No SDKs. No complexity. Save anything, recall it with natural language. Works for humans in the terminal and for AI agents as a skill. Everything runs locally via Ollama — no cloud, no accounts.

```bash
# Save anything
yaad add "staging db is postgres on port 5433"
yaad add "deploy checklist: run migrations, restart workers, clear cache"

# Set a reminder
yaad add "book conference ticket" --remind "in 30 minutes"

# Ask anything
yaad ask "what's the staging db port?"
yaad ask "do I have anything due tonight?"
```

---

## Requirements

- [Ollama](https://ollama.com) running locally

```bash
ollama pull mxbai-embed-large  # embeddings
ollama pull llama3.2:3b        # reasoning (or any chat model you prefer)
```

---

## Installation

**Linux / macOS:**

```bash
curl -fsSL https://yaad.knl.co.in/install.sh | bash
```

**Go install:**

```bash
go install github.com/kunalsin9h/yaad/cmd/yaad@latest
```

**Pre-built binaries** — [GitHub Releases](https://github.com/KunalSin9h/yaad/releases).

---

## Agent Integration

yaad ships as an [Agent Skill](https://github.com/vercel-labs/skills) — compatible with Claude Code, Cursor, Codex CLI, Gemini CLI, and any agent that supports the open skills standard.

```bash
npx skills add kunalsin9h/yaad -g
```

Once installed, your agent saves and recalls memory across sessions automatically.

---

## Reminders

yaad can remind you of things right in your terminal when they're due.

```bash
yaad add "submit PR for review" --remind "tomorrow 9am"
```

See [REMINDERS.md](./REMINDERS.md) for setup (shell hook or systemd daemon).

---

## More

- [COMMANDS.md](./COMMANDS.md) — full command reference (`list`, `get`, `delete`, `config`, all flags)
- [REMINDERS.md](./REMINDERS.md) — reminder setup guide
- [CONFIG.md](./CONFIG.md) — configuration keys, models, notifier options
- [SPEC.md](./SPEC.md) — product specification and architecture

---

## License

MIT © [Kunal Singh](https://github.com/kunalsin9h)
