package unsplash

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"

	"github.com/labi-le/chiasma/pkg/api"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/rs/zerolog"
)

const Name = "unsplash"

var (
	ErrConnectionTimeOut = errors.New("connection timeout")
)

// searchQuery is a var (not const) so tests can point it at an httptest server.
var searchQuery = "https://unsplash.com/napi/search/photos?page=1&per_page=20&query=%s&xp=reset-search-state%%3Aexperiment"

type Unsplash struct {
	log    zerolog.Logger
	client *http.Client
}

func NewUnsplash(log zerolog.Logger, client *http.Client) *Unsplash {
	return &Unsplash{
		log:    log.With().Str("component", "unsplash").Logger(),
		client: api.Client(client),
	}
}

type SearchResult struct {
	Results []Photo `json:"results"`
}

type Photo struct {
	ID     string `json:"id"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Urls   struct {
		Full string `json:"full"`
	} `json:"urls"`
	Premium bool `json:"premium"`
}

//nolint:ireturn // seam: the unsplash provider satisfies searcher.Searcher and yields searcher.Image.
func (u *Unsplash) Search(ctx context.Context, q string, resolution searcher.Resolution) (searcher.Image, error) {
	log := u.log.With().Str("op", "Search").Logger()
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf(searchQuery, url.QueryEscape(q)),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://unsplash.com/s/photos/"+url.QueryEscape(q))
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36 CrKey/1.54.250320")

	log.Trace().Msgf("requesting unsplash search for: %s", q)
	photo, err := u.tryFetch(ctx, req)
	if err != nil {
		return nil, err
	}

	// The CDN resizes the image to the requested dimensions, so the returned
	// bytes may be SMALLER than photo.Width/Height. api.Download reports the
	// ACTUAL decoded size, which is what the service size-gate relies on.
	imgURL := fmt.Sprintf("%s&w=%d&h=%d", photo.Urls.Full, resolution.Width, resolution.Height)
	log.Trace().Msgf("requesting image from unsplash: %s", imgURL)
	img, err := api.Download(ctx, u.client, imgURL)
	if err != nil {
		return nil, timeoutErr(imgURL, err)
	}
	return img, nil
}

func (u *Unsplash) tryFetch(ctx context.Context, req *http.Request) (Photo, error) {
	log := u.log.With().Str("op", "tryFetch").Logger()
	for range 5 {
		if err := ctx.Err(); err != nil {
			return Photo{}, fmt.Errorf("unsplash fetch canceled: %w", err)
		}

		var r SearchResult
		if err := api.FetchJSON(u.client, req, &r); err != nil {
			return Photo{}, timeoutErr(req.URL.String(), err)
		}

		var candidates []Photo
		for _, photo := range r.Results {
			if !photo.Premium {
				candidates = append(candidates, photo)
			}
		}

		if len(candidates) > 0 {
			//nolint:gosec // G404: non-crypto random pick among equivalent free photos; not security-sensitive.
			return candidates[rand.IntN(len(candidates))], nil
		}

		log.Trace().Msg("got a watermarked photo, trying again")
	}

	return Photo{}, errors.New("failed to fetch watermarked photo after multiple attempts")
}

// timeoutErr maps a context deadline into the exported ErrConnectionTimeOut so
// callers can distinguish network timeouts from other failures.
func timeoutErr(u string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: api: %s", ErrConnectionTimeOut, u)
	}
	return err
}
