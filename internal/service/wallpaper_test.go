package service_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/labi-le/chiasma/internal/service"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeImage struct {
	io.Reader
	w, h   int
	closed bool
}

func newFakeImage(body string, w, h int) *fakeImage {
	return &fakeImage{Reader: strings.NewReader(body), w: w, h: h}
}

func (f *fakeImage) Size() (int, int) { return f.w, f.h }
func (f *fakeImage) Close() error     { f.closed = true; return nil }

type fakeAPI struct {
	img    searcher.Image
	err    error
	calls  int
	onCall func(call int) (searcher.Image, error)
}

//nolint:ireturn // fake implements searcher.Searcher, whose Search signature returns the searcher.Image interface.
func (a *fakeAPI) Search(_ context.Context, _ string, _ searcher.Resolution) (searcher.Image, error) {
	a.calls++
	if a.onCall != nil {
		return a.onCall(a.calls)
	}
	if a.err != nil {
		return nil, a.err
	}
	return a.img, nil
}

type fakeSetter struct {
	changed    bool
	path       string
	output     string
	changeErr  error
	closeCalls int
}

func (s *fakeSetter) Change(_ context.Context, path, output string) error {
	s.changed = true
	s.path = path
	s.output = output
	return s.changeErr
}

func (s *fakeSetter) Close(_ context.Context) error { s.closeCalls++; return nil }

type fakeSaver struct {
	called  bool
	path    string
	skipped bool
	err     error
	gotDir  string
	gotTags []string
}

func (f *fakeSaver) save(_ context.Context, data io.Reader, dir string, tags []string) (string, bool, error) {
	f.called = true
	f.gotDir = dir
	f.gotTags = tags
	_, _ = io.Copy(io.Discard, data)
	if f.err != nil {
		return "", false, f.err
	}
	return f.path, f.skipped, nil
}

func baseParams() service.UpdateParams {
	return service.UpdateParams{
		Phrase:     "deep forest",
		Resolution: searcher.Resolution{Width: 100, Height: 100},
		SaveDir:    "/tmp/x",
		OutputID:   "out-0",
		RetryCount: 3,
	}
}

// --- tests ---

func TestUpdateOneShotSuccess(t *testing.T) {
	img := newFakeImage("bytes", 200, 200)
	api := &fakeAPI{img: img}
	setter := &fakeSetter{}
	saver := &fakeSaver{path: "/tmp/x/sw-abc__deep_forest.png"}

	svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: setter, Save: saver.save}

	err := svc.Update(context.Background(), baseParams())
	require.NoError(t, err)

	assert.Equal(t, 1, api.calls)
	assert.True(t, saver.called)
	assert.Equal(t, []string{"deep", "forest"}, saver.gotTags)
	assert.True(t, setter.changed)
	assert.Equal(t, "/tmp/x/sw-abc__deep_forest.png", setter.path)
	assert.Equal(t, "out-0", setter.output)
	assert.True(t, img.closed, "image should be closed")
}

func TestUpdateRetryExhaustion(t *testing.T) {
	api := &fakeAPI{err: errors.New("boom")}
	setter := &fakeSetter{}
	saver := &fakeSaver{}
	svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: setter, Save: saver.save}

	err := svc.Update(context.Background(), baseParams())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Contains(t, err.Error(), "boom")
	assert.Equal(t, 3, api.calls)
	assert.False(t, saver.called)
	assert.False(t, setter.changed)
}

func TestUpdateImageTooSmallRetries(t *testing.T) {
	small := newFakeImage("x", 10, 10)
	api := &fakeAPI{img: small}
	svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: &fakeSetter{}, Save: (&fakeSaver{}).save}

	err := svc.Update(context.Background(), baseParams())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image too small")
	assert.Equal(t, 3, api.calls)
}

func TestUpdateContextCancelledMidRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	api := &fakeAPI{onCall: func(call int) (searcher.Image, error) {
		if call == 1 {
			cancel() // cancel after first failed attempt
		}
		return nil, errors.New("transient")
	}}
	setter := &fakeSetter{}
	svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: setter, Save: (&fakeSaver{}).save}

	err := svc.Update(ctx, baseParams())
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	// Loop short-circuits on the second iteration instead of using all retries.
	assert.Equal(t, 1, api.calls)
	assert.False(t, setter.changed)
}

func TestUpdateRejectsNonPositiveRetryCount(t *testing.T) {
	for _, rc := range []int{0, -1} {
		api := &fakeAPI{}
		svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: &fakeSetter{}, Save: (&fakeSaver{}).save}
		p := baseParams()
		p.RetryCount = rc
		err := svc.Update(context.Background(), p)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "retry count must be positive")
		assert.Equal(t, 0, api.calls, "must reject before calling API")
	}
}

func TestUpdateValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*service.UpdateParams)
		want   string
	}{
		{"empty save dir", func(p *service.UpdateParams) { p.SaveDir = "" }, "save directory is empty"},
		{"whitespace save dir", func(p *service.UpdateParams) { p.SaveDir = "   " }, "save directory is empty"},
		{"zero resolution", func(p *service.UpdateParams) { p.Resolution = searcher.Resolution{} }, "resolution must be positive"},
		{"zero width", func(p *service.UpdateParams) { p.Resolution = searcher.Resolution{Width: 0, Height: 10} }, "resolution must be positive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: &fakeSetter{}, Save: (&fakeSaver{}).save}
			p := baseParams()
			tc.mutate(&p)
			err := svc.Update(context.Background(), p)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Equal(t, 0, api.calls)
		})
	}
}

func TestUpdateEmptyPhraseNoHistory(t *testing.T) {
	api := &fakeAPI{}
	svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: &fakeSetter{}, Save: (&fakeSaver{}).save}
	p := baseParams()
	p.Phrase = "   " // whitespace-only trims to empty
	err := svc.Update(context.Background(), p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no history source")
	assert.Equal(t, 0, api.calls)
}

type fakeHistory struct {
	phrase string
	err    error
}

func (h fakeHistory) GetLastSearch() (string, error) { return h.phrase, h.err }

func TestUpdateUsesHistoryWhenPhraseBlank(t *testing.T) {
	img := newFakeImage("bytes", 200, 200)
	api := &fakeAPI{img: img}
	setter := &fakeSetter{}
	saver := &fakeSaver{path: "/tmp/x/sw-abc.png"}
	svc := &service.WallpaperService{
		Log:     zerolog.Nop(),
		API:     api,
		History: fakeHistory{phrase: "  ocean waves  "},
		Setter:  setter,
		Save:    saver.save,
	}
	p := baseParams()
	p.Phrase = ""
	err := svc.Update(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, []string{"ocean", "waves"}, saver.gotTags)
	assert.True(t, setter.changed)
}

func TestUpdateSaveError(t *testing.T) {
	img := newFakeImage("bytes", 200, 200)
	api := &fakeAPI{img: img}
	setter := &fakeSetter{}
	saver := &fakeSaver{err: errors.New("disk full")}
	svc := &service.WallpaperService{Log: zerolog.Nop(), API: api, Setter: setter, Save: saver.save}

	err := svc.Update(context.Background(), baseParams())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save image")
	assert.Contains(t, err.Error(), "disk full")
	assert.False(t, setter.changed)
}
