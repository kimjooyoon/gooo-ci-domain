package cibudgetdomain

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// One process group is the cancellation unit, including child Go compilers.
// A bounded output buffer prevents a failed command filling local storage.
type limitedOutput struct {
	sync.Mutex
	buffer   bytes.Buffer
	overflow bool
}

func (output *limitedOutput) Write(data []byte) (int, error) {
	originalSize := len(data)
	output.Lock()
	defer output.Unlock()
	remaining := 4*1024*1024 - output.buffer.Len()
	if len(data) > remaining {
		output.overflow = true
		data = data[:max(remaining, 0)]
	}
	output.buffer.Write(data)
	return originalSize, nil
}
func (output *limitedOutput) bytes() ([]byte, bool) {
	output.Lock()
	defer output.Unlock()
	return append([]byte(nil), output.buffer.Bytes()...), output.overflow
}

func managedCommand(ctx context.Context, root, name string, args []string, output *limitedOutput) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = root
	command.Env = controlledEnvironment(os.Environ())
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdout = output
	command.Stderr = output
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	command.WaitDelay = 500 * time.Millisecond
	return command
}
func controlledEnvironment(parent []string) []string {
	result := []string{}
	for _, entry := range parent {
		key, _, _ := strings.Cut(entry, "=")
		if key != "GOMAXPROCS" && key != "GOWORK" && key != "GOTOOLCHAIN" {
			result = append(result, entry)
		}
	}
	return append(result, "GOMAXPROCS=2", "GOWORK=off", "GOTOOLCHAIN=go1.27.2")
}
func commandOutput(ctx context.Context, root, name string, args []string) ([]byte, error) {
	output := &limitedOutput{}
	command := managedCommand(ctx, root, name, args, output)
	errorsOutput := &limitedOutput{}
	command.Stderr = errorsOutput
	err := command.Run()
	if command.Process != nil {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	data, overflow := output.bytes()
	if err != nil {
		return nil, fmt.Errorf("observation command %s failed: %w", name, err)
	}
	if overflow {
		return nil, fmt.Errorf("observation output exceeded 4 MiB")
	}
	return data, nil
}
func resolveCommit(ctx context.Context, root, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("source revision is absent")
	}
	raw, err := commandOutput(ctx, root, "git", []string{"rev-parse", "--verify", "--end-of-options", ref + "^{commit}"})
	return strings.TrimSpace(string(raw)), err
}
func verifyCleanHead(ctx context.Context, root, head string) error {
	actual, err := resolveCommit(ctx, root, "HEAD")
	if err != nil {
		return err
	}
	if actual != head {
		return fmt.Errorf("checkout does not equal requested source revision")
	}
	raw, err := commandOutput(ctx, root, "git", []string{"status", "--porcelain=v1", "--untracked-files=all"})
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) != 0 {
		return fmt.Errorf("checkout has unbound local changes")
	}
	return nil
}
