# Reminders

Set a reminder while saving a memory:

```bash
yaad add "book conference ticket" --remind "in 30 minutes"
yaad add "deploy to prod" --remind "tomorrow 3pm"
yaad add "submit PR for review" --remind "Friday 9am"
```

yaad surfaces reminders right in your terminal when they're due. Two ways to set it up:

## Option 1 — Inline via PROMPT_COMMAND (recommended)

Reminders appear on every shell prompt. No background process needed.

**bash** — add to `~/.bashrc`:

```bash
export PROMPT_COMMAND="yaad check; $PROMPT_COMMAND"
```

**zsh** — add to `~/.zshrc`:

```zsh
precmd() { yaad check }
```

## Option 2 — Background daemon (systemd)

Runs continuously and fires desktop notifications via `notify-send`.

```bash
yaad daemon install
systemctl --user enable --now yaad

# check status
systemctl --user status yaad
```
