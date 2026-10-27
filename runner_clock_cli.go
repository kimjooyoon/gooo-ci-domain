package cibudgetdomain

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

func MainRunnerClock(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("runner-clock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runPath := flags.String("run-json", "", "original GitHub run response")
	jobsPath := flags.String("jobs-json", "", "complete current-attempt GitHub jobs response")
	var binding ClockBinding
	flags.StringVar(&binding.Head, "head", "", "exact source head")
	flags.Int64Var(&binding.RunID, "run-id", 0, "original run identity")
	flags.Int64Var(&binding.Attempt, "attempt", 0, "original run attempt")
	flags.Int64Var(&binding.BudgetMS, "budget-ms", 0, "one total active-runner time allowance")
	flags.StringVar(&binding.CurrentJob, "job", "", "current GitHub job name")
	flags.StringVar(&binding.ProducerJob, "producer", "", "successful tool producer job name")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(stderr, err)
		if _, ok := errors.AsType[*ClockSnapshotPendingError](err); ok {
			return 75
		}
		return 1
	}
	var run ClockRun
	var jobs ClockJobs
	runRaw, err := readClockAPI(*runPath, &run)
	if err != nil {
		return fail(err)
	}
	jobsRaw, err := readClockAPI(*jobsPath, &jobs)
	if err != nil {
		return fail(err)
	}
	report, err := ObserveRunnerClock(run, jobs, binding, time.Now().UTC())
	if err != nil {
		return fail(err)
	}
	report.RunSHA256, report.JobsSHA256 = hashBytes(runRaw), hashBytes(jobsRaw)
	if err = json.NewEncoder(stdout).Encode(report); err != nil {
		return fail(err)
	}
	if report.Decision != "READY" {
		return fail(fmt.Errorf("runner clock: %s", report.Decision))
	}
	return 0
}

// GitHub owns this evolving API shape. Decode the fields used by the clock,
// retaining original bytes separately; domain-owned receipts remain strict.
func readClockAPI(path string, into any) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 4*1024*1024 {
		return nil, fmt.Errorf("runner clock API response exceeds 4 MiB")
	}
	return raw, json.Unmarshal(raw, into)
}
