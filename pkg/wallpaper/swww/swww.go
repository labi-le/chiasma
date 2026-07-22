package swww

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
)

const Name = "swww"

type SWWW struct {
	runner execute.CmdRunner
}

func NewSWWW() (*SWWW, error) {
	if _, err := exec.LookPath(Name); err != nil {
		return nil, fmt.Errorf("%s: %w", Name, execute.ErrUtilityNotFound)
	}

	return &SWWW{runner: execute.ExecRunner{}}, nil
}

// run starts a short-lived command and waits for it to exit, reaping the
// process and releasing the context goroutine.
func (t *SWWW) run(ctx context.Context, args ...string) error {
	cmd := t.runner.Command(ctx, Name, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("swww start: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("swww wait: %w", err)
	}
	return nil
}

func (t *SWWW) Change(ctx context.Context, path, output string) error {
	return t.run(ctx, "img", path, "-o", output)
}

func (t *SWWW) Close(ctx context.Context) error {
	return t.run(ctx, "clear")
}
