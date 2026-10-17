package cibudgetdomain

import (
	"context"
	"flag"
	"fmt"
	"io"
	"time"
)

func MainWork(args []string, stdout, stderr io.Writer) int {
	start := time.Now()
	flags := flag.NewFlagSet("work", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "active working directory")
	id := flags.String("unit", "", "source-owned work identity")
	head := flags.String("head", "HEAD", "exact source head")
	journal := flags.String("journal", "", "external append-only timing journal")
	origin := flags.String("workspace-origin", "", "optional materialization origin file")
	budget := flags.Int64("budget-ms", 180000, "total ceiling")
	started := flags.String("started-at", "", "original workflow creation clock")
	if err := flags.Parse(args); err != nil || flags.NArg() == 0 {
		return 2
	}
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	if *started != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *started)
		if err != nil {
			return fail(err)
		}
		start = parsed
	}
	ctx, cancel, err := BudgetContext(context.Background(), start, *budget)
	if err != nil {
		return fail(err)
	}
	defer cancel()
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	workspace, err := WorkspaceOrigin(*origin)
	if err != nil {
		return fail(err)
	}
	spec, err := ObserveWork(ctx, *root, *id, *head, workspace, flags.Args())
	if err != nil {
		return fail(err)
	}
	if *journal == "" {
		*journal, err = defaultJournal(*root, "command-work")
		if err != nil {
			return fail(err)
		}
	}
	result, err := RunWorkWithOutput(ctx, *root, *journal, spec, start, *budget, stderr)
	return writeWorkResult(stdout, stderr, result, err)
}
