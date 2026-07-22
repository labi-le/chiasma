package config

import (
	"os"
	"time"

	"github.com/labi-le/chiasma/pkg/api/nasa"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/labi-le/chiasma/pkg/browser"
	flag "github.com/spf13/pflag"
)

// defaultRetryCount is the number of image search retries when the flag is unset.
const defaultRetryCount = 5

type Config struct {
	BrowserName    string
	HistoryPath    string
	Resolution     searcher.Resolution
	OutputMonitor  searcher.Monitor
	ToolName       string
	APIName        string
	SaveDir        string
	SearchPhrase   string
	Follow         bool
	FollowDuration time.Duration
	Verbose        bool
	RetryCount     int
}

// Parse reads the process arguments into a Config. On a flag parsing error
// pflag prints usage and terminates the process (ExitOnError), so Parse never
// returns an error to the caller.
func Parse() Config {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	c := register(fs)
	// error is unreachable: ExitOnError terminates the process on failure.
	_ = fs.Parse(os.Args[1:])
	return *c
}

// register binds every flag to a fresh Config on fs and returns it. It is the
// single source of flag definitions, shared by Parse and tests.
func register(fs *flag.FlagSet) *Config {
	c := &Config{}
	fs.StringVar(&c.BrowserName, "browser", browser.AvailableBrowsers()[0], "browser name")
	fs.StringVar(&c.HistoryPath, "history-file", "", "path to history file")
	fs.Var(&c.Resolution, "resolution", "target resolution (e.g. 1920x1080)")
	fs.Var(&c.OutputMonitor, "output", "monitor output (e.g. eDP-1)")
	fs.StringVar(&c.ToolName, "tool", "", "wallpaper tool")
	fs.StringVar(&c.APIName, "api", nasa.Name, "image source api")
	fs.StringVar(&c.SaveDir, "save-dir", os.Getenv("HOME")+"/Pictures/chiasma", "save directory")
	fs.StringVar(&c.SearchPhrase, "phrase", "", "search phrase")
	fs.DurationVar(&c.FollowDuration, "interval", time.Hour, "update interval")
	fs.BoolVar(&c.Follow, "follow", false, "enable periodic updates")
	fs.BoolVar(&c.Verbose, "verbose", false, "enable verbose logs")
	fs.IntVar(&c.RetryCount, "retry-count", defaultRetryCount, "number of search retries")
	return c
}
