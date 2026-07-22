//nolint:testpackage // white-box: constructs SWWW with the unexported runner field.
package swww

import (
	"context"
	"errors"
	"testing"

	"github.com/labi-le/chiasma/pkg/wallpaper/execute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCmd struct {
	name     string
	args     []string
	started  bool
	waited   bool
	killed   bool
	startErr error
}

func (c *fakeCmd) Start() error { c.started = true; return c.startErr }
func (c *fakeCmd) Wait() error  { c.waited = true; return nil }
func (c *fakeCmd) Kill() error  { c.killed = true; return nil }

type fakeRunner struct {
	cmds     []*fakeCmd
	startErr error
}

//nolint:ireturn // test fake implements the CmdRunner seam, must return Cmd.
func (r *fakeRunner) Command(_ context.Context, name string, args ...string) execute.Cmd {
	c := &fakeCmd{name: name, args: args, startErr: r.startErr}
	r.cmds = append(r.cmds, c)
	return c
}

func TestChangeBuildsImgArgsAndWaits(t *testing.T) {
	r := &fakeRunner{}
	sut := &SWWW{runner: r}

	require.NoError(t, sut.Change(context.Background(), "/pics/a.jpg", "DP-1"))

	require.Len(t, r.cmds, 1)
	c := r.cmds[0]
	assert.Equal(t, "swww", c.name)
	assert.Equal(t, []string{"img", "/pics/a.jpg", "-o", "DP-1"}, c.args)
	assert.True(t, c.started, "process must be started")
	assert.True(t, c.waited, "short-lived client must be waited on (no leak)")
	assert.False(t, c.killed)
}

func TestCloseBuildsClearArgsAndWaits(t *testing.T) {
	r := &fakeRunner{}
	sut := &SWWW{runner: r}

	require.NoError(t, sut.Close(context.Background()))

	require.Len(t, r.cmds, 1)
	c := r.cmds[0]
	assert.Equal(t, "swww", c.name)
	assert.Equal(t, []string{"clear"}, c.args)
	assert.True(t, c.started)
	assert.True(t, c.waited)
}

func TestChangePropagatesStartError(t *testing.T) {
	wantErr := errors.New("boom")
	r := &fakeRunner{startErr: wantErr}
	sut := &SWWW{runner: r}

	err := sut.Change(context.Background(), "/pics/a.jpg", "DP-1")
	require.ErrorIs(t, err, wantErr)
	assert.False(t, r.cmds[0].waited, "must not wait if start failed")
}
