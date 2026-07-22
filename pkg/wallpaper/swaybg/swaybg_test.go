//nolint:testpackage // white-box: constructs SwayBG with unexported runner/cmd fields.
package swaybg

import (
	"context"
	"testing"

	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCmd struct {
	name    string
	args    []string
	started bool
	waited  bool
	killed  bool
}

func (c *fakeCmd) Start() error { c.started = true; return nil }
func (c *fakeCmd) Wait() error  { c.waited = true; return nil }
func (c *fakeCmd) Kill() error  { c.killed = true; return nil }

type fakeRunner struct {
	cmds []*fakeCmd
}

//nolint:ireturn // test fake implements the CmdRunner seam, must return Cmd.
func (r *fakeRunner) Command(_ context.Context, name string, args ...string) execute.Cmd {
	c := &fakeCmd{name: name, args: args}
	r.cmds = append(r.cmds, c)
	return c
}

func TestChangeStartsDaemonWithArgs(t *testing.T) {
	r := &fakeRunner{}
	sut := &SwayBG{runner: r}

	require.NoError(t, sut.Change(context.Background(), "/pics/a.jpg", "DP-1"))

	require.Len(t, r.cmds, 1)
	c := r.cmds[0]
	assert.Equal(t, "swaybg", c.name)
	assert.Equal(t, []string{"-i", "/pics/a.jpg", "-o", "DP-1"}, c.args)
	assert.True(t, c.started, "daemon must be started")
	assert.False(t, c.waited, "daemon must not be waited on synchronously")
	assert.False(t, c.killed, "daemon must stay alive after Change")
	assert.Same(t, execute.Cmd(c), sut.cmd, "daemon handle must be retained")
}

func TestCloseTerminatesAndReapsDaemon(t *testing.T) {
	r := &fakeRunner{}
	sut := &SwayBG{runner: r}

	require.NoError(t, sut.Change(context.Background(), "/pics/a.jpg", "DP-1"))
	require.NoError(t, sut.Close(context.Background()))

	c := r.cmds[0]
	assert.True(t, c.killed, "Close must terminate the daemon")
	assert.True(t, c.waited, "Close must reap the daemon (no zombie)")
	assert.Nil(t, sut.cmd, "handle cleared after Close")
}

func TestCloseIsNoopWithoutDaemon(t *testing.T) {
	r := &fakeRunner{}
	sut := &SwayBG{runner: r}

	require.NoError(t, sut.Close(context.Background()))
	assert.Empty(t, r.cmds)
}

func TestChangeReplacesAndReapsPreviousDaemon(t *testing.T) {
	r := &fakeRunner{}
	sut := &SwayBG{runner: r}

	require.NoError(t, sut.Change(context.Background(), "/pics/a.jpg", "DP-1"))
	require.NoError(t, sut.Change(context.Background(), "/pics/b.jpg", "DP-2"))

	require.Len(t, r.cmds, 2)
	first, second := r.cmds[0], r.cmds[1]
	assert.True(t, first.killed, "previous daemon must be terminated on replace")
	assert.True(t, first.waited, "previous daemon must be reaped on replace")
	assert.False(t, second.killed, "new daemon must stay alive")
	assert.Same(t, execute.Cmd(second), sut.cmd)
}
