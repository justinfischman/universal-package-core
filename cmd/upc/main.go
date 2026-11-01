// Command upc creates, installs and manages packages and apps across languages.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/justinfischman/universal-package-core/internal/cli"
	"github.com/justinfischman/universal-package-core/internal/core"
	"github.com/justinfischman/universal-package-core/internal/register"
)

// Set at link time: -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	regs, err := register.Default()
	if err != nil {
		fmt.Fprintf(os.Stderr, "upc: build registry: %v\n", err)
		return core.ExitFailure
	}
	deps := cli.DefaultDeps(regs, cli.BuildInfo{Version: version, Commit: commit, Date: date})
	return cli.Execute(ctx, deps, os.Args[1:])
}
