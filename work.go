package cibudgetdomain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type WorkSpec struct {
	WorkingDirectory                                                                    string
	ID, SourceHead, SourceTree, PolicyDigest, WorkspaceOrigin, Profile, CostKey, Digest string
	ExecutionID                                                                         string
	Argv                                                                                []string
	Executable, ExecutableDigest                                                        string
}
type WorkReceipt struct {
	Schema, Decision, NextOperation           string
	Spec                                      WorkSpec
	Admission                                 string
	PredictionMS                              *int64
	UnitElapsedMS, ScopeElapsedMS, DeadlineMS int64
	ExitCode                                  int
	Completed                                 bool
	OutputDigest                              string
	OutputBytes                               int64
	Observation                               *Observation
}

func ObserveWork(ctx context.Context, root, id, head, workspaceOrigin string, argv []string) (WorkSpec, error) {
	return ObserveWorkWithExecutionID(ctx, root, id, head, workspaceOrigin, "", argv)
}

// ObserveWorkWithExecutionID separates the current execution from reusable
// source and cost cohorts. An empty ID retains the original single-origin rule.
func ObserveWorkWithExecutionID(ctx context.Context, root, id, head, workspaceOrigin,
	executionID string, argv []string) (WorkSpec, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return WorkSpec{}, err
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return WorkSpec{}, err
	}
	result := WorkSpec{WorkingDirectory: hashBytes([]byte(absolute)), ID: id, Argv: append([]string(nil), argv...),
		WorkspaceOrigin: workspaceOrigin, ExecutionID: executionID, PolicyDigest: SourcePolicyDigest()}
	if id == "" || len(id) > 1024 || len(executionID) > 128 ||
		strings.ContainsAny(executionID, "\r\n\x00") || len(argv) == 0 || len(argv) > 128 {
		return result, fmt.Errorf("work identity and exact argument population are required")
	}
	argumentBytes := 0
	for _, argument := range argv {
		argumentBytes += len(argument)
		if strings.ContainsRune(argument, 0) {
			return result, fmt.Errorf("argument contains a null byte")
		}
	}
	if argumentBytes > 65536 {
		return result, fmt.Errorf("command arguments exceed 64 KiB")
	}
	observed, err := resolveCommit(ctx, root, "HEAD")
	if err != nil {
		return result, err
	}
	requested, err := resolveCommit(ctx, root, head)
	if err != nil {
		return result, err
	}
	if observed != requested {
		return result, fmt.Errorf("work source head differs")
	}
	result.SourceHead = observed
	// Respect the caller's logical Git index. Untracked command outputs do not
	// change source identity, but tracked inputs must match the declared revision.
	if _, err = commandOutput(ctx, root, "git", []string{"diff", "--exit-code", "HEAD", "--"}); err != nil {
		return result, fmt.Errorf("active tracked source differs: %w", err)
	}
	result.Executable, result.ExecutableDigest, err = observeExecutable(ctx, absolute, argv[0])
	if err != nil {
		return result, err
	}
	tree, err := commandOutput(ctx, root, "git", []string{"rev-parse", observed + "^{tree}"})
	if err != nil {
		return result, err
	}
	result.SourceTree = strings.TrimSpace(string(tree))
	environment, err := commandOutput(ctx, root, "go", []string{"env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT"})
	if err != nil {
		return result, err
	}
	result.Profile = hashBytes(append(environment, []byte(fmt.Sprintf("command-wall/v1;GOMAXPROCS=2;GOWORK=off;cpu=%d", runtime.NumCPU()))...))
	definition, _ := json.Marshal(struct {
		ID, Profile, WorkingDirectory, ExecutableDigest string
		Argv                                            []string
	}{id, result.Profile, result.WorkingDirectory, result.ExecutableDigest, argv})
	result.CostKey = hashBytes(definition)
	sealed, _ := json.Marshal(result)
	result.Digest = hashBytes(sealed)
	return result, nil
}

func RunWork(ctx context.Context, root, journal string, spec WorkSpec, started time.Time, budget int64) (WorkReceipt, error) {
	return RunWorkWithOutput(ctx, root, journal, spec, started, budget, io.Discard)
}

func RunWorkWithOutput(ctx context.Context, root, journal string, spec WorkSpec, started time.Time, budget int64, log io.Writer) (WorkReceipt, error) {
	result := WorkReceipt{Schema: "gooo/command-work-origin/v1", Spec: spec, DeadlineMS: budget, ExitCode: -1}
	bounded, cancel, err := BudgetContext(ctx, started, budget)
	if err != nil {
		return result, err
	}
	defer cancel()
	ctx = bounded
	clone := spec
	clone.Digest = ""
	sealed, _ := json.Marshal(clone)
	if spec.Digest != hashBytes(sealed) || spec.PolicyDigest != SourcePolicyDigest() {
		return result, fmt.Errorf("command source definition differs")
	}
	history, err := ReadObservations(journal)
	if err != nil {
		return result, err
	}
	estimates, err := Estimates(history)
	if err != nil {
		return result, err
	}
	novel := true
	for _, old := range history {
		if old.Unit == spec.ID && old.SourceTree == spec.SourceTree && old.Profile == spec.Profile &&
			old.ExecutionID == spec.ExecutionID && (old.CostKey == spec.CostKey || old.CostKey == "reservation:"+spec.CostKey) {
			novel = false
		}
	}
	prefix := time.Since(started).Milliseconds()
	estimate, known := estimates[spec.CostKey]
	known = known && estimate.Samples > 0 && (estimate.Censored == 0 || estimate.ObservedMaxMS > estimate.CensoredFloorMS)
	if known {
		forecast := estimate.ForecastMS
		result.PredictionMS = &forecast
		result.Admission = AdmitPlan(forecast, prefix, budget, true, true)
	} else {
		result.Admission = AdmitDiscovery(budget, prefix, true, novel)
	}
	if !novel {
		return result, fmt.Errorf("this exact source command has an existing origin")
	}
	if result.Admission != "READY" && result.Admission != "MEASURE_NEW_COHORT" {
		result.Decision = result.Admission
		result.NextOperation = NextStep(result.Decision)
		result.ScopeElapsedMS = prefix
		return result, nil
	}
	reservation := Observation{ID: hashBytes([]byte(spec.SourceTree+":"+spec.CostKey+":"+spec.ExecutionID)) + "/start",
		CostKey: spec.CostKey, SourceDigest: spec.Digest, SourceTree: spec.SourceTree, Profile: spec.Profile,
		Unit: spec.ID, ExecutionID: spec.ExecutionID, Outcome: "STARTED", PredictionState: "unavailable"}
	// A reservation is an execution identity, not a zero-time command cost sample.
	reservation.CostKey = "reservation:" + spec.CostKey
	if err = AppendObservations(journal, []Observation{reservation}); err != nil {
		return result, err
	}
	fresh, err := ObserveWorkWithExecutionID(ctx, root, spec.ID, spec.SourceHead,
		spec.WorkspaceOrigin, spec.ExecutionID, spec.Argv)
	if err != nil || fresh.Digest != spec.Digest {
		return result, fmt.Errorf("source or command context changed before execution")
	}
	begin := time.Now()
	output := &workOutput{sink: log, hash: sha256.New()}
	command := managedCommand(ctx, root, spec.Executable, spec.Argv[1:], &limitedOutput{})
	command.Args[0] = spec.Argv[0]
	command.Stdout = output
	command.Stderr = output
	commandErr := command.Run()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	output.Lock()
	digest := hex.EncodeToString(output.hash.Sum(nil))
	captured := output.failure == nil
	result.OutputBytes = output.bytes
	output.Unlock()
	result.UnitElapsedMS = time.Since(begin).Milliseconds()
	result.ScopeElapsedMS = time.Since(started).Milliseconds()
	result.Completed = ctx.Err() == nil && command.Process != nil
	result.OutputDigest = digest
	result.ExitCode = 0
	if commandErr != nil {
		result.ExitCode = 1
		if failure, ok := commandErr.(*exec.ExitError); ok {
			result.ExitCode = failure.ExitCode()
		}
	}
	result.Decision = ClassifyWorkOutcome(ctx.Err() != nil, int64(result.ExitCode), captured && command.Process != nil)
	result.NextOperation = NextStep(result.Decision)
	observation := Observation{ID: spec.Digest + "/end/" + strconv.FormatInt(begin.UnixNano(), 10),
		CostKey: spec.CostKey, SourceDigest: spec.Digest, SourceTree: spec.SourceTree, Profile: spec.Profile,
		Unit: spec.ID, ExecutionID: spec.ExecutionID, Outcome: result.Decision,
		ElapsedMS: result.UnitElapsedMS, ResolutionMS: 1, Completed: result.Completed, OutputDigest: result.OutputDigest, PredictionState: "unavailable"}
	if known {
		observation.PredictionState = "empirical"
		observation.PredictedMS = estimate.ForecastMS
	}
	result.Observation = &observation
	if err = AppendObservations(journal, []Observation{observation}); err != nil {
		return result, err
	}
	result.ScopeElapsedMS = time.Since(started).Milliseconds()
	if ctx.Err() != nil {
		result.Decision = "DEADLINE_EXCEEDED"
		result.NextOperation = NextStep(result.Decision)
	}
	return result, nil
}

func WorkspaceOrigin(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 4*1024*1024 {
		return "", fmt.Errorf("workspace origin exceeds 4 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(file, 4*1024*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 4*1024*1024 {
		return "", fmt.Errorf("workspace origin exceeds 4 MiB")
	}
	return hashBytes(data), nil
}

func observeExecutable(ctx context.Context, root, name string) (string, string, error) {
	path := name
	var err error
	if strings.ContainsRune(name, os.PathSeparator) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
	} else {
		path, err = exec.LookPath(name)
		if err != nil {
			return "", "", err
		}
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 512*1024*1024 {
		return "", "", fmt.Errorf("executable origin is unavailable or exceeds 512 MiB")
	}
	digest := sha256.New()
	buffer := make([]byte, 32768)
	var size int64
	for {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		n, err := file.Read(buffer)
		size += int64(n)
		if size > 512*1024*1024 {
			return "", "", fmt.Errorf("executable origin exceeds 512 MiB")
		}
		_, _ = digest.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}
	}
	return path, hex.EncodeToString(digest.Sum(nil)), nil
}
