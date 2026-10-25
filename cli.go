package cibudgetdomain

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "runner-clock" {
		return MainRunnerClock(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "work" {
		return MainWork(args[1:], stdout, stderr)
	}
	start := time.Now()
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: gooo-ci-domain collect|plan|check|improve|verify|work|runner-clock [flags]")
		return 2
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "exact source checkout")
	base := flags.String("base", "", "source base commit")
	head := flags.String("head", "HEAD", "source head commit")
	input := flags.String("input", "", "optional collected repository JSON")
	journal := flags.String("journal", "", "optional external timing journal; default uses bounded system cache")
	budget := flags.Int64("budget-ms", 180000, "total source-owned ceiling")
	workers := flags.Int("workers", 1, "available package workers")
	started := flags.String("started-at", "", "RFC3339Nano workflow start, including queue/setup")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return 2
	}
	if *started != "" {
		parsed, err := time.Parse(time.RFC3339Nano, *started)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		start = parsed
	}
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	emit := func(value any) int {
		if err := json.NewEncoder(stdout).Encode(value); err != nil {
			return fail(err)
		}
		return 0
	}
	ctx, cancel, err := BudgetContext(context.Background(), start, *budget)
	if err != nil {
		return fail(err)
	}
	defer cancel()
	if ctx.Err() != nil {
		return fail(ctx.Err())
	}
	if args[0] == "verify" {
		if err := RunBounded(ctx, *root, "go", []string{"test", "-race", "-mod=readonly", "-p=1", "-count=1", "-timeout=180s", "./..."}); err != nil {
			return fail(err)
		}
		return emit(map[string]any{"decision": "VERIFIED_DOMAIN_TOOL", "scope_elapsed_ms": time.Since(start).Milliseconds(), "deadline_ms": *budget})
	}
	if args[0] != "collect" && args[0] != "plan" && args[0] != "check" && args[0] != "improve" {
		return fail(fmt.Errorf("unknown operation %q", args[0]))
	}
	var repository Repository
	if *input == "" {
		repository, err = CollectWithWorkers(ctx, *root, *base, *head, *workers)
		if err != nil {
			return fail(err)
		}
		repository.ScopeStartedAt = start
	} else {
		if err = ReadJSON(*input, &repository); err != nil {
			return fail(err)
		}
		if err = repository.Validate(); err != nil {
			return fail(err)
		}
		if repository.ScopeStartedAt.Before(start) {
			start = repository.ScopeStartedAt
			cancel()
			ctx, cancel, err = BudgetContext(context.Background(), start, *budget)
			if err != nil {
				return fail(err)
			}
			defer cancel()
		}
	}
	if args[0] == "collect" {
		return emit(repository)
	}
	if *workers != repository.Workers {
		return fail(fmt.Errorf("worker capacity differs from collected cost profile"))
	}
	if *journal == "" {
		*journal, err = defaultJournal(*root, repository.Module)
		if err != nil {
			return fail(err)
		}
	}
	history, err := ReadObservations(*journal)
	if err != nil {
		return fail(err)
	}
	plan, _, _, err := ObservedPlan(repository, history, time.Since(start).Milliseconds(), *budget)
	if err != nil {
		return fail(err)
	}
	if args[0] == "plan" {
		return emit(struct {
			Plan          Plan
			NextOperation string
		}{plan, NextStep(plan.Decision)})
	}
	if plan.Decision == "READY" || args[0] == "improve" && plan.Decision == "COST_UNRESOLVED" {
		for _, old := range history {
			if old.SourceDigest == repository.Digest || old.SourceTree == repository.Tree && old.Profile == repository.Profile {
				return fail(fmt.Errorf("source cohort has an existing origin; inspect it before changing relevant source"))
			}
		}
		reservation := Observation{ID: repository.Digest + "/cohort-start", CostKey: "reservation:" + repository.Digest,
			SourceDigest: repository.Digest, SourceTree: repository.Tree, Profile: repository.Profile,
			Unit: "cohort-start", Outcome: "STARTED", PredictionState: "unavailable"}
		if err = AppendObservations(*journal, []Observation{reservation}); err != nil {
			return fail(err)
		}
	}
	var result Execution
	if args[0] == "improve" {
		result, err = Improve(ctx, *root, repository, history, start, *budget)
	} else if plan.Decision != "READY" {
		result = Execution{Decision: plan.Decision, NextOperation: NextStep(plan.Decision), SourceDigest: repository.Digest, Plan: plan}
	} else {
		result, err = Execute(ctx, *root, repository, plan, history)
	}
	if err != nil {
		return fail(err)
	}
	if len(result.Observations) > 0 {
		if err = AppendObservations(*journal, result.Observations); err != nil {
			return fail(err)
		}
	}
	result.ScopeElapsedMS = time.Since(start).Milliseconds()
	if ctx.Err() != nil {
		result.Decision = "DEADLINE_EXCEEDED"
		result.NextOperation = NextStep(result.Decision)
	}
	if code := emit(result); code != 0 {
		return code
	}
	if result.Decision != "VERIFIED_SCOPED" && result.Decision != "VERIFIED_DISCOVERY" {
		return 1
	}
	return 0
}
func defaultJournal(root, module string) (string, error) {
	directory, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	directory = filepath.Join(directory, "gooo-ci-domain", hashBytes([]byte(module+"\x00"+absolute)))
	if err = os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	return filepath.Join(directory, "origins.jsonl"), nil
}
func ReadJSON(name string, value any) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 4*1024*1024 {
		return fmt.Errorf("input exceeds 4 MiB")
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("input has trailing data")
	}
	return nil
}
