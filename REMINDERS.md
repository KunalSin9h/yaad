# Reminders

`yaad` can remind you about anything at a natural language time:

```bash
yaad add "book conference ticket" --remind "in 30 minutes"
yaad add "submit PR for review"   --remind "tomorrow 9am"
yaad add "deploy to prod"         --remind "Friday 3pm"
```

There are two ways to surface reminders — pick what fits your workflow.

---

## Option 1 — Inline via `PROMPT_COMMAND` (recommended)

Reminders surface directly in your terminal on every prompt. No background process, no dependencies.

**bash** — add to `~/.bashrc`:
```bash
export PROMPT_COMMAND="yaad check; $PROMPT_COMMAND"
```

**zsh** — add to `~/.zshrc`:
```zsh
precmd() { yaad check }
```

When a reminder is due, `yaad check` prints a styled box inline and marks it as fired. Silent otherwise — zero cost on every prompt.

---

## Option 2 — Background daemon via systemd

Installs as a systemd user service. Fires desktop notifications via `notify-send` (requires `libnotify-bin`).

```bash
yaad daemon install                       # writes ~/.config/systemd/user/yaad.service
systemctl --user enable --now yaad        # start and enable on login

systemctl --user status yaad              # check status
systemctl --user disable --now yaad       # remove
```

Poll interval defaults to 30 seconds. Override in `~/.yaadrc`:

```ini
reminder.poll_interval = 10s
```

---

## Notifiers

| Adapter | Platform | Requirement |
|---|---|---|
| `cli` | All | None — prints a styled box to the terminal |
| `notify-send` | Linux | `notify-send` must be installed (`libnotify-bin`) |

Both can fire together:

```bash
yaad config set notifier cli,notify-send
```
