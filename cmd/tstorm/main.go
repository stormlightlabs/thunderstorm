// Command tstorm runs the thunderstorm workflow's checks and board operations.
//
// The skills a harness loads are prose, and prose cannot fail a run. Everything
// here is the other half: the checks that hold the loop to what the prose says,
// and the board writes that are too racy to leave to a model.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/stormlightlabs/tstorm/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, cli.ErrUsage) {
			fmt.Fprintln(os.Stderr, "tstorm:", err)
		}
		os.Exit(1)
	}
}
