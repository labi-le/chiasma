package nasa //nolint:testpackage // white-box: overrides the unexported nasaSearchURL to target an httptest server.

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

// nasaServer wires the three-hop NASA flow (search -> asset -> image) against a
// single httptest mux. imgW/imgH size the served image.
func nasaServer(t *testing.T, imgW, imgH int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var base string
	srv := httptest.NewServer(mux)
	base = srv.URL

	mux.HandleFunc("/search", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"collection":{"items":[{"href":"` + base + `/asset","data":[{"title":"nebula"}]}]}}`))
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`["` + base + `/img~orig.jpg"]`))
	})
	mux.HandleFunc("/img~orig.jpg", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(pngBytes(t, imgW, imgH))
	})
	return srv
}

func withSearchURL(srv *httptest.Server) func() {
	prev := nasaSearchURL
	nasaSearchURL = srv.URL + "/search?q=%s"
	return func() { nasaSearchURL = prev }
}

func newTestNasa() *Nasa {
	return NewNasa(zerolog.Nop(), &http.Client{Timeout: 5 * time.Second})
}

func TestNasaSearchSuccess(t *testing.T) {
	srv := nasaServer(t, 200, 200)
	defer srv.Close()
	defer withSearchURL(srv)()

	img, err := newTestNasa().Search(context.Background(), "space", searcher.Resolution{Width: 100, Height: 100})
	require.NoError(t, err)
	defer img.Close()

	w, h := img.Size()
	assert.Equal(t, 200, w)
	assert.Equal(t, 200, h)
}

func TestNasaSearchNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestNasa().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestNasaSearchBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{not valid`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestNasa().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestNasaSearchEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"collection":{"items":[]}}`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestNasa().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestNasaSearchTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	defer withSearchURL(srv)()

	n := NewNasa(zerolog.Nop(), &http.Client{Timeout: 20 * time.Millisecond})
	_, err := n.Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

// TestNasaSearchRejectsBadAspectRatio verifies the aspect-ratio gate rejects a
// panorama/strip when it is the only candidate.
func TestNasaSearchRejectsBadAspectRatio(t *testing.T) {
	srv := nasaServer(t, 1000, 100) // ratio 10 -> rejected
	defer srv.Close()
	defer withSearchURL(srv)()

	_, err := newTestNasa().Search(context.Background(), "space", searcher.Resolution{})
	require.Error(t, err)
}

func TestNasaSearchContextCanceled(t *testing.T) {
	srv := nasaServer(t, 200, 200)
	defer srv.Close()
	defer withSearchURL(srv)()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newTestNasa().Search(ctx, "space", searcher.Resolution{})
	require.Error(t, err)
}
