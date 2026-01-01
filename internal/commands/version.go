package commands

import (
	"context"
	"fmt"

	"github.com/mikael.mansson2/drime-shell/internal/build"
	"github.com/mikael.mansson2/drime-shell/internal/session"
)

func init() {
	Register(&Command{
		Name:        "version",
		Description: "Print version information",
		Usage:       "version",
		Run:         versionCmd,
	})
}

func versionCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fmt.Fprintf(env.Stdout, "drime-shell version %s\n", build.Version)
	fmt.Fprintf(env.Stdout, "Commit: %s\n", build.Commit)
	fmt.Fprintf(env.Stdout, "Date:   %s\n", build.Date)
	return nil
}
