package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/kunalsin9h/yaad/internal/adapters/notifier"
	"github.com/kunalsin9h/yaad/internal/adapters/ollama"
	"github.com/kunalsin9h/yaad/internal/adapters/rcfile"
	sqliteadapter "github.com/kunalsin9h/yaad/internal/adapters/sqlite"
	"github.com/kunalsin9h/yaad/internal/adapters/timeparser"
	"github.com/kunalsin9h/yaad/internal/app"
	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
	"github.com/spf13/cobra"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	// Determine paths.
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rcPath := filepath.Join(home, ".yaadrc")
	dataDir, err := dataDirectory()
	if err != nil {
		return err
	}

	// Load rc file (missing file is fine — all keys fall back to defaults).
	rc := rcfile.New(rcPath)

	// Services are built inside PersistentPreRunE, after flags are parsed,
	// so that CLI flags can override rc file values.
	// Commands access them via closure over these variables.
	var (
		memorySvc   *app.MemoryService
		reminderSvc *app.ReminderService
		db          *sqliteadapter.DB
	)

	root := &cobra.Command{
		Use:   "yaad",
		Short: "AI-native memory, recall and reminder on the terminal",
		Long: `yaad — save anything from your terminal, recall it with natural language.

Examples:
  yaad add "claude --resume abc123" --for "yaad build session"
  yaad add "book conference ticket" --remind "in 30 minutes"
  yaad ask "which claude session was I building yaad in?"
  yaad list`,

		// Build all services here — flags have been parsed, rc file is loaded.
		// Config priority: built-in defaults < ~/.yaadrc < CLI flags.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip for config and help commands — they don't need AI or DB.
			if cmd.Name() == "init" || cmd.Name() == "help" || cmd.Name() == "completion" {
				return nil
			}

			var err error
			db, err = sqliteadapter.Open(filepath.Join(dataDir, "memories.db"))
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}

			// Resolve: flag > rc file > built-in default.
			ollamaURL := resolve(cmd, "ollama-url", rc, "ollama.url", "http://localhost:11434")
			embedModel := resolve(cmd, "embed-model", rc, "ollama.embed_model", "mxbai-embed-large")
			chatModel := resolve(cmd, "chat-model", rc, "ollama.chat_model", "llama3.2:3b")

			aiClient := ollama.New(ollamaURL, embedModel, chatModel)

			notif := buildNotifier(resolve(cmd, "notifier", rc, "notifier", "cli"))

			memorySvc = app.NewMemoryService(db.Store, aiClient, timeparser.New())
			reminderSvc = app.NewReminderService(db.Store, notif)
			return nil
		},
	}

	// Root persistent flags — override rc file for any sub-command.
	root.PersistentFlags().String("ollama-url", "", "Ollama server URL (overrides rc file)")
	root.PersistentFlags().String("chat-model", "", "Chat model to use (overrides rc file)")
	root.PersistentFlags().String("embed-model", "", "Embedding model to use (overrides rc file)")
	root.PersistentFlags().String("notifier", "", "Notifier adapter: cli|notify-send (overrides rc file)")

	root.AddCommand(
		addCmd(&memorySvc, &db),
		askCmd(&memorySvc),
		listCmd(&memorySvc),
		getCmd(&memorySvc),
		deleteCmd(&memorySvc),
		cleanCmd(&memorySvc),
		checkCmd(&reminderSvc),
		daemonCmd(&reminderSvc, rc),
		configCmd(rc, rcPath),
	)

	return root.Execute()
}

// resolve returns the first non-empty value in: CLI flag → rc file → default.
func resolve(cmd *cobra.Command, flagName string, rc ports.ConfigPort, rcKey, defaultVal string) string {
	// CLI flag takes highest priority (only if explicitly set by the user).
	if f := cmd.Root().PersistentFlags().Lookup(flagName); f != nil && f.Changed {
		return f.Value.String()
	}
	// rc file next.
	if v, _ := rc.Get(rcKey); v != "" {
		return v
	}
	return defaultVal
}

// --- add ---

func addCmd(svc **app.MemoryService, db **sqliteadapter.DB) *cobra.Command {
	var forLabel, remind string

	cmd := &cobra.Command{
		Use:   "add <content>",
		Short: "Save a new memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer (*db).Close()
			m, err := (*svc).Add(context.Background(), app.AddRequest{
				Content:    args[0],
				ForLabel:   forLabel,
				RemindExpr: remind,
			})
			if err != nil {
				return err
			}
			fmt.Printf("saved    %s\n", shortID(m.ID))
			if m.RemindAt != nil {
				fmt.Printf("remind   %s\n", relTime(*m.RemindAt))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&forLabel, "for", "f", "", "Context: why are you saving this?")
	cmd.Flags().StringVar(&remind, "remind", "", `When to remind you, e.g. "in 30 minutes", "tomorrow 9am"`)
	return cmd
}

// --- ask ---

func askCmd(svc **app.MemoryService) *cobra.Command {
	return &cobra.Command{
		Use:   "ask <question>",
		Short: "Query your memories with natural language",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			answer, err := (*svc).Ask(context.Background(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			fmt.Println(answer)
			return nil
		},
	}
}

// --- list ---

func listCmd(svc **app.MemoryService) *cobra.Command {
	var limit int
	var remindOnly bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			memories, err := (*svc).List(context.Background(), domain.ListFilter{
				Limit:         limit,
				OnlyReminders: remindOnly,
			})
			if err != nil {
				return err
			}
			if len(memories) == 0 {
				fmt.Println("no memories found")
				return nil
			}
			printTable(memories)
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")
	cmd.Flags().BoolVar(&remindOnly, "remind", false, "Show only pending reminders")
	return cmd
}

// --- get ---

func getCmd(svc **app.MemoryService) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show full details of a memory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := (*svc).GetByID(context.Background(), args[0])
			if err != nil {
				return err
			}
			printDetail(m)
			return nil
		},
	}
}

// --- delete ---

func deleteCmd(svc **app.MemoryService) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a memory by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Printf("delete %s? [y/N] ", shortID(args[0]))
				var ans string
				fmt.Scanln(&ans) //nolint:errcheck
				if strings.ToLower(strings.TrimSpace(ans)) != "y" {
					fmt.Println("cancelled")
					return nil
				}
			}
			if err := (*svc).Delete(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Printf("deleted %s\n", shortID(args[0]))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "y", false, "Skip confirmation prompt")
	return cmd
}

// --- clean ---

func cleanCmd(svc **app.MemoryService) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Delete all memories",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Print("delete ALL memories? this cannot be undone. [y/N] ")
				var ans string
				fmt.Scanln(&ans) //nolint:errcheck
				if strings.ToLower(strings.TrimSpace(ans)) != "y" {
					fmt.Println("cancelled")
					return nil
				}
			}
			n, err := (*svc).Clean(context.Background())
			if err != nil {
				return err
			}
			fmt.Printf("deleted %d memories\n", n)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "y", false, "Skip confirmation prompt")
	return cmd
}

// --- check ---

func checkCmd(svc **app.ReminderService) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check for due reminders (silent unless reminders are firing)",
		Long: `Designed to run on every shell prompt via PROMPT_COMMAND:

  Add to ~/.bashrc or ~/.zshrc:
    export PROMPT_COMMAND="yaad check; $PROMPT_COMMAND"

  For zsh, add to ~/.zshrc:
    precmd() { yaad check }`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return (*svc).CheckAndFire(context.Background())
		},
	}
}

// --- daemon ---

func daemonCmd(svc **app.ReminderService, rc ports.ConfigPort) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the background reminder daemon",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "start",
			Short: "Start the daemon in the foreground (use systemd for background)",
			RunE: func(cmd *cobra.Command, args []string) error {
				intervalStr, _ := rc.Get("reminder.poll_interval")
				if intervalStr == "" {
					intervalStr = "30s"
				}
				interval, err := time.ParseDuration(intervalStr)
				if err != nil {
					interval = 30 * time.Second
				}
				ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
				defer stop()
				fmt.Printf("daemon started  poll-interval=%s  Ctrl+C to stop\n", interval)
				return (*svc).RunDaemon(ctx, interval)
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Install as a systemd user service",
			RunE: func(cmd *cobra.Command, args []string) error {
				return installSystemdService()
			},
		},
	)
	return cmd
}

// --- config ---

func configCmd(rc *rcfile.Config, rcPath string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage yaad configuration (~/.yaadrc)",
		// Config subcommands do not need the DB or AI client.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "init",
			Short: "Create ~/.yaadrc with commented defaults",
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := rc.Init(); err != nil {
					return err
				}
				fmt.Printf("created %s\n", rcPath)
				fmt.Println("edit it with your preferred text editor to configure yaad.")
				fmt.Println()
				fmt.Println("tip: yaad can remind you about memories.")
				fmt.Println("     to set up CLI reminders, see: https://github.com/KunalSin9h/yaad/blob/main/docs/REMINDERS.md")
				return nil
			},
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set a config value in ~/.yaadrc",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				if err := rc.Set(args[0], args[1]); err != nil {
					return err
				}
				fmt.Printf("set %s = %s  (in %s)\n", args[0], args[1], rcPath)
				return nil
			},
		},
		&cobra.Command{
			Use:   "get <key>",
			Short: "Get a config value",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				v, err := rc.Get(args[0])
				if err != nil {
					return err
				}
				if v == "" {
					fmt.Println("(not set — using default)")
				} else {
					fmt.Println(v)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all values in ~/.yaadrc",
			RunE: func(cmd *cobra.Command, args []string) error {
				all, err := rc.All()
				if err != nil {
					return err
				}
				if len(all) == 0 {
					fmt.Printf("no config set in %s — all defaults in use.\n", rcPath)
					fmt.Println("run: yaad config init   to create a config file")
					return nil
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for k, v := range all {
					fmt.Fprintf(w, "%s\t%s\n", k, v)
				}
				return w.Flush()
			},
		},
		&cobra.Command{
			Use:   "path",
			Short: "Print the path to the config file",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(rcPath)
				return nil
			},
		},
	)
	return cmd
}

// --- output helpers ---

func printTable(memories []*domain.Memory) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCONTENT\tFOR\tCREATED")
	fmt.Fprintln(w, "--\t-------\t---\t-------")
	for _, m := range memories {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			shortID(m.ID),
			truncate(m.Content, 50),
			truncate(m.ForLabel, 30),
			relTime(m.CreatedAt),
		)
	}
	w.Flush()
}

func printDetail(m *domain.Memory) {
	fmt.Printf("ID       : %s\n", m.ID)
	fmt.Printf("Content  : %s\n", m.Content)
	if m.ForLabel != "" {
		fmt.Printf("For      : %s\n", m.ForLabel)
	}
	fmt.Printf("Created  : %s (%s)\n", m.CreatedAt.Format(time.RFC822), relTime(m.CreatedAt))
	if m.WorkingDir != "" {
		fmt.Printf("Dir      : %s\n", m.WorkingDir)
	}
	if m.Hostname != "" {
		fmt.Printf("Host     : %s\n", m.Hostname)
	}
	if m.RemindAt != nil {
		fmt.Printf("Remind at: %s (%s)\n", m.RemindAt.Format(time.RFC822), relTime(*m.RemindAt))
	}
	if m.RemindedAt != nil {
		fmt.Printf("Reminded : %s\n", m.RemindedAt.Format(time.RFC822))
	}
}

func shortID(id string) string {
	if len(id) > 10 {
		return id[:10]
	}
	return id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func relTime(t time.Time) string {
	d := time.Since(t)
	future := d < 0
	if future {
		d = -d
	}
	var s string
	switch {
	case d < time.Minute:
		if future {
			return "in a moment"
		}
		return "just now"
	case d < time.Hour:
		s = fmt.Sprintf("%d min", int(d.Minutes()))
	case d < 24*time.Hour:
		s = fmt.Sprintf("%d hr", int(d.Hours()))
	default:
		s = fmt.Sprintf("%d days", int(d.Hours()/24))
	}
	if future {
		return "in " + s
	}
	return s + " ago"
}

// buildNotifier parses a comma-separated list of notifier names and returns
// a Multi notifier that fans out to all of them.
func buildNotifier(cfg string) ports.NotifierPort {
	var ns []ports.NotifierPort
	for name := range strings.SplitSeq(cfg, ",") {
		switch strings.TrimSpace(name) {
		case "notify-send":
			ns = append(ns, notifier.NewNotifySend())
		default: // "cli" or anything unrecognised
			ns = append(ns, notifier.NewCLI())
		}
	}
	if len(ns) == 0 {
		ns = append(ns, notifier.NewCLI())
	}
	return notifier.NewMulti(ns...)
}

func dataDirectory() (string, error) {
	var base string
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		base = xdg
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "yaad")
	return dir, os.MkdirAll(dir, 0o700)
}

func installSystemdService() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	service := fmt.Sprintf(`[Unit]
Description=yaad reminder daemon
After=graphical-session.target

[Service]
ExecStart=%s daemon start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
`, execPath)

	svcDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(svcDir, 0o755); err != nil {
		return err
	}
	svcPath := filepath.Join(svcDir, "yaad.service")
	if err := os.WriteFile(svcPath, []byte(service), 0o644); err != nil {
		return err
	}
	fmt.Printf("installed: %s\n", svcPath)
	fmt.Println("enable  : systemctl --user enable --now yaad")
	return nil
}
