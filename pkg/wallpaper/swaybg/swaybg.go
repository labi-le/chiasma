package swaybg

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
)

const Name = "swaybg"

type SwayBG struct {
	runner execute.CmdRunner

	mu  sync.Mutex
	cmd execute.Cmd
}

func NewSwayBG() (*SwayBG, error) {
	if _, err := exec.LookPath(Name); err != nil {
		return nil, fmt.Errorf("%s: %w", Name, execute.ErrUtilityNotFound)
	}

	return &SwayBG{runner: execute.ExecRunner{}}, nil
}

func (t *SwayBG) Change(_ context.Context, path, output string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Replace any previous daemon, reaping it so it does not linger.
	if err := t.terminate(); err != nil {
		return err
	}

	// swaybg is a long-lived daemon: its lifetime is decoupled from the
	// request context so the wallpaper persists after Change returns.
	// Termination is explicit, via Close or the next Change.
	cmd := t.runner.Command(context.Background(), Name, "-i", path, "-o", output)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("swaybg start: %w", err)
	}
	t.cmd = cmd
	return nil
}

func (t *SwayBG) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.terminate()
}

// terminate kills and reaps the current daemon (if any). Caller holds t.mu.
func (t *SwayBG) terminate() error {
	if t.cmd == nil {
		return nil
	}
	err := t.cmd.Kill()
	_ = t.cmd.Wait() // reap to avoid a zombie and release the ctx goroutine
	t.cmd = nil
	if err != nil {
		return fmt.Errorf("swaybg terminate: %w", err)
	}
	return nil
}
