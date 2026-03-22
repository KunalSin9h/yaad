# Command Reference

## `yaad add` — Save a memory

```bash
yaad add "<content>"
yaad add "<content>" --remind "when"
```

Put context directly in the content — the AI embeds the full string, so searchable context belongs there:

```bash
# good — self-contained, findable later
yaad add "staging db is postgres on port 5433"
yaad add "prod login: ssh -i ~/.ssh/id_rsa user@bastion.internal"
yaad add "API rate limit is 100 req/min per token"
yaad add "deploy checklist: run migrations, restart workers, clear cache"
yaad add "submit PR for review" --remind "tomorrow 9am"

# avoid — no context, won't recall well
yaad add "port 5433"
yaad add "check this later"
```

## `yaad ask` — Recall with natural language

```bash
yaad ask "what's the staging db port?"
yaad ask "how do I log into prod?"
yaad ask "do I have anything due tonight?"
```

## `yaad list` — Browse memories

```bash
yaad list                   # 20 most recent
yaad list --remind          # pending reminders only
yaad list --limit 50
```

## `yaad get` — Full details

```bash
yaad get 01KKXKKJ3Q         # 10-char ULID prefix is enough
```

## `yaad delete` / `yaad clean` — Remove memories

```bash
yaad delete 01KKXKKJ3Q      # prompts for confirmation
yaad delete 01KKXKKJ3Q -y   # skip confirmation
yaad clean                  # delete all memories
```

## `yaad config` — Manage configuration

```bash
yaad config init            # write ~/.yaadrc with commented defaults
yaad config set ollama.chat_model mistral
yaad config list
```

See [CONFIG.md](./CONFIG.md) for all keys, notifier options, and CLI flag overrides.
