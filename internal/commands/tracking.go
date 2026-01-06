package commands

import (
	"context"
	"fmt"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

func init() {
	Register(&Command{
		Name:        "track",
		Description: "Manage file tracking",
		Usage: `track [command] <file>...

Commands:
  track ls                List all tracked files
  track <file>...         Start tracking files
  track stats <file>      Show tracking statistics
  track off <file>...     Stop tracking (alias for untrack)

Options:
  -s, --stats             Show tracking statistics
  -l, --list              List all tracked files
  --off                   Stop tracking

Examples:
  track file.pdf          Start tracking
  track ls                List tracked files
  track stats file.pdf    Show stats`,
		Run: track,
	})
}

func track(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// Handle subcommands first
	if len(args) > 0 {
		switch args[0] {
		case "ls", "list":
			return listTrackedFiles(ctx, s, env, args[1:])
		case "stats":
			if len(args) < 2 {
				return fmt.Errorf("usage: track stats <file>")
			}
			// Resolve path and show stats
			path := args[1]
			resolvedPath := s.ResolvePath(path)
			entry, ok := s.Cache.Get(resolvedPath)
			if !ok {
				return fmt.Errorf("file not found: %s", path)
			}
			return showTrackingStats(ctx, s, env, entry)
		case "off":
			if len(args) < 2 {
				return fmt.Errorf("usage: track off <file>")
			}
			return untrack(ctx, s, env, args[1:])
		}
	}

	flags := pflag.NewFlagSet("track", pflag.ContinueOnError)
	showStats := flags.BoolP("stats", "s", false, "Show tracking statistics")
	listTracked := flags.BoolP("list", "l", false, "List all tracked files")
	off := flags.Bool("off", false, "Stop tracking")
	flags.SetOutput(env.Stderr)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *listTracked {
		return listTrackedFiles(ctx, s, env, nil)
	}

	if flags.NArg() == 0 {
		return fmt.Errorf("usage: track [options] <file>")
	}

	if *off {
		return untrack(ctx, s, env, flags.Args())
	}

	path := flags.Arg(0)
	resolvedPath := s.ResolvePath(path)
	entry, ok := s.Cache.Get(resolvedPath)
	if !ok {
		return fmt.Errorf("file not found: %s", path)
	}

	if *showStats {
		return showTrackingStats(ctx, s, env, entry)
	}

	// Enable tracking
	for _, arg := range flags.Args() {
		p := s.ResolvePath(arg)
		e, ok := s.Cache.Get(p)
		if !ok {
			fmt.Fprintf(env.Stderr, "file not found: %s\n", arg)
			continue
		}
		if err := s.Client.SetTracking(ctx, e.ID, true); err != nil {
			fmt.Fprintf(env.Stderr, "failed to track %s: %v\n", arg, err)
		} else {
			fmt.Fprintf(env.Stdout, "Tracking enabled for %s\n", e.Name)
		}
	}
	return nil
}

func untrack(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: untrack <file>")
	}

	for _, arg := range args {
		p := s.ResolvePath(arg)
		e, ok := s.Cache.Get(p)
		if !ok {
			fmt.Fprintf(env.Stderr, "file not found: %s\n", arg)
			continue
		}
		if err := s.Client.SetTracking(ctx, e.ID, false); err != nil {
			fmt.Fprintf(env.Stderr, "failed to untrack %s: %v\n", arg, err)
		} else {
			fmt.Fprintf(env.Stdout, "Tracking disabled for %s\n", e.Name)
		}
	}
	return nil
}

func listTrackedFiles(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	files, err := s.Client.GetTrackedFiles(ctx)
	if err != nil {
		return err
	}

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("NAME"),
		ui.HeaderStyle.Render("VIEWS"),
		ui.HeaderStyle.Render("DOWNLOADS"),
	)
	for _, f := range files {
		t.AddRow(f.Name, fmt.Sprintf("%d", f.ViewsNumber), fmt.Sprintf("%d", f.DlNumber))
	}
	t.Render()
	return nil
}

func showTrackingStats(ctx context.Context, s *session.Session, env *ExecutionEnv, entry *api.FileEntry) error {
	stats, err := s.Client.GetTrackingStats(ctx, entry.ID)
	if err != nil {
		return err
	}

	fmt.Fprintf(env.Stdout, "Tracking stats for %s:\n", entry.Name)

	if len(stats.Views) == 0 {
		fmt.Fprintln(env.Stdout, "No events recorded.")
		return nil
	}

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("DATE"),
		ui.HeaderStyle.Render("ACTION"),
		ui.HeaderStyle.Render("IP"),
		ui.HeaderStyle.Render("LOCATION"),
		ui.HeaderStyle.Render("USER"),
	)
	for _, v := range stats.Views {
		user := "-"
		if v.Email != nil {
			user = *v.Email
		}
		t.AddRow(v.Date, v.Action, v.IP, v.Location, user)
	}
	t.Render()
	return nil
}
