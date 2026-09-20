// Command tstorm runs the thunderstorm workflow's checks and board operations.
//
// A coding agent reads the workflow as instructions and can skip one without
// leaving a record. The checks here exit non-zero instead. The board commands
// are here for a different reason: they are compare-and-swap writes against
// GitHub Projects, which an agent has no way to perform.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/stormlightlabs/tstorm/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// cobra is told to stay silent so that one place decides how an error
	// reaches the operator: on stderr, prefixed, and never on stdout, which
	// a caller may be parsing.
	if err := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "tstorm:", err)
		os.Exit(1)
	}
}
