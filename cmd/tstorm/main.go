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

	"github.com/stormlightlabs/thunderstorm/internal/cli"
)

func main() {
	// NotifyContext, with nothing reading the context, replaced SIGINT's
	// default with nothing at all and made Ctrl-C a no-op. A render is short,
	// so the default is right: die. A render killed part way leaves a staging
	// directory beside the destination, which the next render ignores; killed
	// inside the rename that swaps the payload in, it can leave the previous
	// payload under a .tstorm-previous- name to move back by hand.
	ctx := context.Background()

	// cobra is told to stay silent so that one place decides how an error
	// reaches the operator: on stderr, prefixed, and never on stdout, which
	// a caller may be parsing.
	if err := cli.Execute(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "tstorm:", err)
		os.Exit(1)
	}
}
