package config //nolint:testpackage // white-box: tests drive the unexported register() flag-binding helper

import (
	"testing"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseArgs registers the flags on an isolated FlagSet (ContinueOnError so a
// bad flag returns an error instead of terminating the test process) and parses
// the supplied args, mirroring what Parse does for os.Args.
func parseArgs(t *testing.T, args []string) (Config, error) {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := register(fs)
	err := fs.Parse(args)
	return *c, err
}

func TestParseDefaults(t *testing.T) {
	c, err := parseArgs(t, nil)
	require.NoError(t, err)

	assert.Equal(t, 5, c.RetryCount, "default retry count")
	assert.Equal(t, time.Hour, c.FollowDuration, "default interval")
	assert.False(t, c.Follow)
	assert.False(t, c.Verbose)
	assert.Empty(t, c.SearchPhrase)
	assert.Empty(t, c.ToolName)
	assert.Equal(t, 0, c.Resolution.Width)
	assert.Equal(t, 0, c.Resolution.Height)
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		assert func(t *testing.T, c Config)
	}{
		{
			name: "retry count override",
			args: []string{"--retry-count", "12"},
			assert: func(t *testing.T, c Config) {
				assert.Equal(t, 12, c.RetryCount)
			},
		},
		{
			name: "resolution wiring",
			args: []string{"--resolution", "1920x1080"},
			assert: func(t *testing.T, c Config) {
				assert.Equal(t, 1920, c.Resolution.Width)
				assert.Equal(t, 1080, c.Resolution.Height)
			},
		},
		{
			// Monitor.Set validates against xrandr, so a live parse is
			// environment-dependent; assert the flag defaults to an empty ID.
			name: "monitor output default empty",
			args: nil,
			assert: func(t *testing.T, c Config) {
				assert.Empty(t, c.OutputMonitor.ID)
			},
		},
		{
			name: "follow and interval",
			args: []string{"--follow", "--interval", "30m"},
			assert: func(t *testing.T, c Config) {
				assert.True(t, c.Follow)
				assert.Equal(t, 30*time.Minute, c.FollowDuration)
			},
		},
		{
			name: "verbose and phrase",
			args: []string{"--verbose", "--phrase", "mountains"},
			assert: func(t *testing.T, c Config) {
				assert.True(t, c.Verbose)
				assert.Equal(t, "mountains", c.SearchPhrase)
			},
		},
		{
			name: "api, tool and save dir",
			args: []string{"--api", "local", "--tool", "swww", "--save-dir", "/tmp/pics"},
			assert: func(t *testing.T, c Config) {
				assert.Equal(t, "local", c.APIName)
				assert.Equal(t, "swww", c.ToolName)
				assert.Equal(t, "/tmp/pics", c.SaveDir)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := parseArgs(t, tt.args)
			require.NoError(t, err)
			tt.assert(t, c)
		})
	}
}

func TestParseInvalidResolution(t *testing.T) {
	_, err := parseArgs(t, []string{"--resolution", "not-a-res"})
	require.Error(t, err)
}

// TestRegisterBindings proves each flag is registered and bound to the matching
// Config field, without invoking validators that need external tools.
func TestRegisterBindings(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	c := register(fs)

	require.NotNil(t, fs.Lookup("output"), "output flag registered")
	assert.Same(t, &c.OutputMonitor, fs.Lookup("output").Value, "output bound to OutputMonitor")

	require.NotNil(t, fs.Lookup("resolution"), "resolution flag registered")
	assert.Same(t, &c.Resolution, fs.Lookup("resolution").Value, "resolution bound to Resolution")

	require.NotNil(t, fs.Lookup("retry-count"), "retry-count flag registered")
	assert.Equal(t, "5", fs.Lookup("retry-count").DefValue, "retry-count default 5")
}
