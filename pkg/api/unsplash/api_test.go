package unsplash //nolint:testpackage // white-box: overrides the unexported searchQuery to target an httptest server.

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func withSearchURL(srv *httptest.Server) func() {
	prev := searchQuery
	searchQuery = srv.URL + "/search?q=%s"
	return func() { searchQuery = prev }
}

func newTestUnsplash() *Unsplash {
	return NewUnsplash(zerolog.Nop(), &http.Client{Timeout: 5 * time.Second})
}

// TestUnsplashSearchSizeFix is the core regression: the search JSON advertises a
// 6000x4000 original, but the CDN serves a resized 1620x1080 image. Size() MUST
// report the ACTUAL downloaded dimensions so the service size-gate works.
func TestUnsplashSearchSizeFix(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()
	base := srv.URL

	mux.HandleFunc("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"results":[{"id":"a","width":6000,"height":4000,"premium":false,"urls":{"full":"` + base + `/img?src=1"}}]}`))
	})
	mux.HandleFunc("/img", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(pngBytes(t, 1620, 1080))
	})
	defer withSearchURL(srv)()

	img, err := newTestUnsplash().Search(context.Background(), "space", searcher.Resolution{Width: 1920, Height: 1080})
	require.NoError(t, err)
	defer img.Close()

	w, h := img.Size()
	assert.Equal(t, 1620, w, "must report actual downloaded width, not 6000")
	assert.Equal(t, 1080, h)
}

func TestUnsplashSearchNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestUnsplash().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestUnsplashSearchBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{oops`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestUnsplash().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestUnsplashSearchAllPremium(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"results":[{"id":"a","premium":true,"urls":{"full":"x"}}]}`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestUnsplash().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestUnsplashSearchTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	u := NewUnsplash(zerolog.Nop(), &http.Client{Timeout: 20 * time.Millisecond})
	_, err := u.Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestUnsplashSearchContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newTestUnsplash().Search(ctx, "space", searcher.Resolution{})
	require.Error(t, err)
}
