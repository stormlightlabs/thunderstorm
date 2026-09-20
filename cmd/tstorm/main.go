// Command tstorm runs the thunderstorm workflow's checks and board operations.
//
// The skills a harness loads are prose, and prose cannot fail a run. Everything
// here is the other half: the checks that hold the loop to what the prose says,
// and the board writes that are too racy to leave to a model.
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
