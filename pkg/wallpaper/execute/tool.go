package execute

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

var (
	ErrUtilityNotFound = errors.New("utility not found in PATH")
)

// Provider drives a concrete wallpaper backend.
type Provider interface {
	// Change sets the wallpaper at path on the given output.
	Change(ctx context.Context, path, output string) error
	// Close releases backend resources (clears state / terminates daemon).
	Close(ctx context.Context) error
}

// Cmd is a handle to a single (possibly long-lived) process. It is the seam
// backends run against so they can be tested without spawning real processes.
type Cmd interface {
	// Start begins execution without blocking.
	Start() error
	// Wait blocks until the process exits, reaping it (no zombie) and
	// releasing the goroutine associated with context cancellation.
	Wait() error
	// Kill terminates the process. It is safe to call before Start returns
	// successfully or after the process has already exited.
	Kill() error
}

// CmdRunner constructs commands. Backends depend on this rather than os/exec
// directly, allowing a fake to be injected in tests.
type CmdRunner interface {
	Command(ctx context.Context, name string, args ...string) Cmd
}

// ExecRunner is the default CmdRunner backed by os/exec.
type ExecRunner struct{}

// Command returns the Cmd interface: it is the injection seam backends run
// against, so returning the interface (not *execCmd) is intentional.
//
//nolint:ireturn // CmdRunner seam: callers depend on the Cmd interface.
func (ExecRunner) Command(ctx context.Context, name string, args ...string) Cmd {
	return &execCmd{cmd: exec.CommandContext(ctx, name, args...)}
}

type execCmd struct {
	cmd *exec.Cmd
}

func (c *execCmd) Start() error {
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func (c *execCmd) Wait() error {
	if err := c.cmd.Wait(); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	return nil
}

func (c *execCmd) Kill() error {
	if c.cmd.Process == nil {
		return nil
	}
	if err := c.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill: %w", err)
	}
	return nil
}
