package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/labi-le/chiasma/internal/config"
	"github.com/labi-le/chiasma/internal/service"
	"github.com/labi-le/chiasma/pkg/api/registry"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/labi-le/chiasma/pkg/browser"
	"github.com/labi-le/chiasma/pkg/wallpaper"
	"github.com/rs/zerolog"
)

func main() {
	cfg := config.Parse()

	log := initLogger(cfg.Verbose)

	srchr, err := registry.NewSearcher(log, cfg.APIName, cfg.SaveDir)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init api")
	}

	tool, err := wallpaper.ByNameOrAvailable(cfg.ToolName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to init wallpaper tool")
	}

	resolution := resolveResolution(log, &cfg)

	// History init registers a cleanup defer, so it runs only after every
	// fatal path above has been cleared (otherwise log.Fatal would skip it).
	historyProvider, closeHistory := initHistory(log, &cfg)
	defer closeHistory()

	svc := &service.WallpaperService{
		Log:     log,
		API:     srchr,
		History: historyProvider,
		Setter:  tool,
	}

	params := service.UpdateParams{
		Phrase:     cfg.SearchPhrase,
		Resolution: resolution,
		SaveDir:    cfg.SaveDir,
		OutputID:   cfg.OutputMonitor.ID,
		RetryCount: cfg.RetryCount,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	defer func() {
		// Detach from ctx so cleanup still runs after a shutdown signal has
		// already cancelled it.
		if cerr := tool.Close(context.WithoutCancel(ctx)); cerr != nil {
			log.Warn().Err(cerr).Msg("failed to close wallpaper tool")
		}
	}()

	run := func() {
		if uerr := svc.Update(ctx, params); uerr != nil {
			log.Error().Err(uerr).Msg("failed to update wallpaper")
		}
	}

	run()

	if !cfg.Follow {
		return
	}

	watch(ctx, log, cfg.FollowDuration, run)
}

// initHistory builds the browser history query source unless an explicit search
// phrase was supplied. It returns the provider (nil when unavailable) and a
// cleanup func that is always safe to call.
//
//nolint:ireturn // seam: returns the service.QuerySource interface wired into WallpaperService.History; NewHistoryProvider has no single concrete type
func initHistory(log zerolog.Logger, cfg *config.Config) (service.QuerySource, func()) {
	noop := func() {}
	if cfg.SearchPhrase != "" {
		return nil, noop
	}

	hp, err := browser.NewHistoryProvider(cfg.BrowserName, cfg.HistoryPath)
	if err != nil {
		log.Warn().Err(err).Msg("failed to init browser history, fallback to random or manual phrase might fail")
		return nil, noop
	}

	closer, ok := hp.(interface{ Close() error })
	if !ok {
		return hp, noop
	}

	return hp, func() {
		if cerr := closer.Close(); cerr != nil {
			log.Warn().Err(cerr).Msg("failed to close browser history")
		}
	}
}

// resolveResolution returns the configured resolution, detecting it from the
// active monitor when the user did not specify one.
func resolveResolution(log zerolog.Logger, cfg *config.Config) searcher.Resolution {
	resolution := cfg.Resolution
	if resolution.Width != 0 && resolution.Height != 0 {
		return resolution
	}

	mon, err := searcher.NewByIDXrandr(cfg.OutputMonitor.ID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to detect resolution, please specify --image-resolution")
	}

	log.Info().Str("res", mon.CurrentResolution.String()).Msg("detected resolution")

	return mon.CurrentResolution
}

// watch runs the update loop on the follow interval until the context is done.
func watch(ctx context.Context, log zerolog.Logger, interval time.Duration, run func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Info().Dur("interval", interval).Msg("entering watch mode")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("shutting down")
			return
		case <-ticker.C:
			run()
		}
	}
}

func initLogger(verbose bool) zerolog.Logger {
	out := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

	if verbose {
		// Shorten the caller to basename:line on this writer instead of
		// mutating the global zerolog.CallerMarshalFunc.
		out.FormatCaller = func(i any) string {
			s, _ := i.(string)
			if idx := strings.LastIndexByte(s, '/'); idx >= 0 {
				s = s[idx+1:]
			}
			return s + " >"
		}
		return zerolog.New(out).
			Level(zerolog.TraceLevel).
			With().
			Timestamp().
			Caller().
			Logger()
	}

	return zerolog.New(out).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Logger()
}
