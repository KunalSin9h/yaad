---
name: yaad
description: yaad is a simple local memory engine for humans and agents. Use it to remember anything worth keeping across sessions, and to recall it later in natural language. Invoke when starting work to load prior context, when something is worth saving, or when the user asks about something from the past.
argument-hint: "[query] or [add <content>] or [add <content> --remind 'when']"
allowed-tools: Bash
---

yaad is a CLI-based AI memory engine — the simplest way to save anything and retrieve it later in natural language. One command to store, one to recall. Everything stays on the user's machine, no cloud, no accounts.

## Check yaad is installed

```bash
which yaad
```

If not found, tell the user to install it first:

```bash
curl -fsSL https://yaad.knl.co.in/install.sh | bash
```

Then stop — do not proceed until yaad is available.

---

## Commands

### Save a memory

```bash
yaad add "<content>"
```

### Save with a reminder

```bash
yaad add "<content>" --remind "in 30 minutes"
yaad add "<content>" --remind "tomorrow 9am"
```

### Recall with natural language

```bash
yaad ask "<question>"
```

---

## When to save

Save proactively when you encounter:

- A command that solved a non-obvious problem
- A port, hostname, or infrastructure detail the user will look up again
- A decision made about architecture, approach, or tooling
- A URL for docs or an API being actively used
- A time-based reminder the user sets
- Any fact the user explicitly says they want to remember

## Writing good memories

Put all context directly in the content — the AI embeds the full string, so searchable context belongs there:

```bash
# good — self-contained, findable later
yaad add "staging db is postgres on port 5433"
yaad add "prod uses nginx, config at /etc/nginx/sites-enabled/app"
yaad add "API rate limit is 100 req/min per token"
yaad add "deploy checklist: run migrations, restart workers, clear cache"
yaad add "standup at 10am" --remind "tomorrow 9:45am"

# avoid — no context, won't recall well
yaad add "port 5433"
yaad add "check this later"
```

## Memory-first instinct

Before reaching for a tool (filesystem search, web search, re-asking the user), ask yourself: **could this already be in memory?**

If the answer is "maybe" — ask yaad first. It's cheaper than a search and respects what the user has already told you.

```bash
yaad ask "<what you're looking for>"
```

After finding or figuring something out that the user will likely need again, save it without being asked.

## When to recall

- At the start of a session, to surface relevant prior context
- Whenever the user asks about something they may have mentioned before
- Before any search — file, web, or otherwise — for facts, paths, configs, or decisions
