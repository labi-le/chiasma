package nasa

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"

	"github.com/labi-le/chiasma/pkg/api"
	"github.com/labi-le/chiasma/pkg/api/searcher"
	"github.com/rs/zerolog"
)

const Name = "nasa"

// nasaSearchURL is a var (not const) so tests can point it at an httptest server.
var nasaSearchURL = "https://images-api.nasa.gov/search?q=%s&media_type=image"

// errBadAspectRatio marks a candidate rejected by the aspect-ratio gate so the
// Search loop can skip it and try the next one.
var errBadAspectRatio = errors.New("image rejected: bad aspect ratio")

var (
	stopWords = []string{
		// Technical terms
		"chart", "diagram", "plot", "spectrum", "graph",
		"schematic", "profile", "response", "model", "map",
		"histogram", "curve", "data", "survey", "sensor",
		// Visual formats irrelevant for wallpapers
		"mosaic", "panorama", "composite",
		// Sources that often produce "scientific strips" (like PIA05979)
		"galex", "evolution explorer",
	}
)

type Nasa struct {
	log    zerolog.Logger
	client *http.Client
}

func NewNasa(log zerolog.Logger, client *http.Client) *Nasa {
	return &Nasa{
		log:    log.With().Str("component", "nasa").Logger(),
		client: api.Client(client),
	}
}

type nasaData struct {
	Title       string   `json:"title"`
	Keywords    []string `json:"keywords"`
	Description string   `json:"description"`
}

type nasaItem struct {
	Href string     `json:"href"`
	Data []nasaData `json:"data"`
}

type nasaSearchResult struct {
	Collection struct {
		Items []nasaItem `json:"items"`
	} `json:"collection"`
}

//nolint:ireturn // seam: the nasa provider satisfies searcher.Searcher and yields searcher.Image.
func (n *Nasa) Search(ctx context.Context, q string, res searcher.Resolution) (searcher.Image, error) {
	log := n.log.With().Str("op", "Search").Logger()

	items, err := n.fetchSearchResults(ctx, q)
	if err != nil {
		return nil, err
	}

	candidates := n.filterCandidates(items)

	if len(candidates) == 0 {
		log.Warn().Msg("all images were filtered out as technical, falling back to raw results")
		candidates = items
	} else {
		log.Info().Int("total", len(items)).Int("clean", len(candidates)).Msg("filtering complete")
	}

	// maxRetries caps how many distinct candidates we try per Search call.
	// This is a per-item fallback (each iteration is a DIFFERENT candidate, not a
	// retry of the same request); the service layer owns outer retries, so we keep
	// this small to avoid an outer*inner request blow-up.
	const maxRetries = 3
	shuffled := make([]nasaItem, len(candidates))
	copy(shuffled, candidates)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	for i := 0; i < len(shuffled) && i < maxRetries; i++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("nasa search canceled: %w", ctxErr)
		}

		selected := shuffled[i]
		log.Debug().Str("href", selected.Href).Int("attempt", i+1).Msg("trying candidate")

		img, tryErr := n.tryCandidate(ctx, selected, res)
		if tryErr == nil {
			return img, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("nasa search canceled: %w", ctxErr)
		}
		log.Warn().Err(tryErr).Msg("candidate rejected")
	}

	return nil, fmt.Errorf("failed to find suitable image after %d attempts", maxRetries)
}

// tryCandidate resolves, downloads, and aspect-checks a single candidate. It
// returns a non-nil error when the candidate must be skipped.
//
//nolint:ireturn // seam: yields the searcher.Image abstraction consumed by Search.
func (n *Nasa) tryCandidate(ctx context.Context, item nasaItem, res searcher.Resolution) (searcher.Image, error) {
	imgURL, err := n.resolveImageURL(ctx, item.Href)
	if err != nil {
		return nil, fmt.Errorf("resolve image url: %w", err)
	}

	img, err := n.downloadImage(ctx, imgURL)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}

	w, h := img.Size()
	if isAspectRatioBad(w, h, res.Width, res.Height) {
		_ = img.Close()
		return nil, errBadAspectRatio
	}

	return img, nil
}

func isAspectRatioBad(imgW, imgH, targetW, targetH int) bool {
	if targetW == 0 || targetH == 0 {
		ratio := float64(imgW) / float64(imgH)
		return ratio > 2.5 || ratio < 0.4
	}

	targetRatio := float64(targetW) / float64(targetH)
	imgRatio := float64(imgW) / float64(imgH)

	diff := math.Abs(targetRatio - imgRatio)
	return diff > 1.0
}

func (n *Nasa) filterCandidates(items []nasaItem) []nasaItem {
	candidates := make([]nasaItem, 0, len(items))

	for _, item := range items {
		clean, reason := n.inspect(item)

		if len(item.Data) > 0 {
			meta := item.Data[0]
			if !clean {
				n.log.Debug().
					Str("title", meta.Title).
					Str("reject_reason", reason).
					Msg("candidate rejected")
			}
		}

		if clean {
			candidates = append(candidates, item)
		}
	}
	return candidates
}

func (n *Nasa) inspect(item nasaItem) (bool, string) {
	if len(item.Data) == 0 {
		return true, ""
	}

	meta := item.Data[0]
	title := strings.ToLower(meta.Title)
	desc := strings.ToLower(meta.Description)

	if found, word := containsStopWord(title); found {
		return false, "title_" + word
	}

	if found, word := containsStopWord(desc); found {
		return false, "desc_" + word
	}

	for _, k := range meta.Keywords {
		kLower := strings.ToLower(k)
		if found, word := containsStopWord(kLower); found {
			return false, "kw_" + word
		}
	}

	return true, ""
}

func containsStopWord(text string) (bool, string) {
	for _, stop := range stopWords {
		if !strings.Contains(text, stop) {
			continue
		}

		if stop == "graph" && (strings.Contains(text, "photograph") || strings.Contains(text, "graphic")) {
			continue
		}

		return true, stop
	}
	return false, ""
}

func (n *Nasa) fetchSearchResults(ctx context.Context, q string) ([]nasaItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(nasaSearchURL, url.QueryEscape(q)), nil)
	if err != nil {
		return nil, fmt.Errorf("create req: %w", err)
	}

	var res nasaSearchResult
	if fetchErr := api.FetchJSON(n.client, req, &res); fetchErr != nil {
		return nil, fmt.Errorf("fetch search results: %w", fetchErr)
	}

	if len(res.Collection.Items) == 0 {
		return nil, fmt.Errorf("no results for %s", q)
	}

	return res.Collection.Items, nil
}

func (n *Nasa) resolveImageURL(ctx context.Context, href string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, href, nil)
	if err != nil {
		return "", fmt.Errorf("create asset req: %w", err)
	}

	var assets []string
	if fetchErr := api.FetchJSON(n.client, req, &assets); fetchErr != nil {
		return "", fmt.Errorf("fetch asset manifest: %w", fetchErr)
	}

	best := findBestImage(assets)
	if best == "" {
		return "", errors.New("no suitable image url found")
	}
	return best, nil
}

//nolint:ireturn // seam: yields the searcher.Image abstraction consumed by Search.
func (n *Nasa) downloadImage(ctx context.Context, url string) (searcher.Image, error) {
	img, err := api.Download(ctx, n.client, url)
	if err != nil {
		return nil, fmt.Errorf("download nasa image: %w", err)
	}
	return img, nil
}

func findBestImage(urls []string) string {
	for _, u := range urls {
		if strings.Contains(u, "~orig.jpg") {
			return u
		}
	}
	for _, u := range urls {
		if strings.Contains(u, "~large.jpg") {
			return u
		}
	}
	if len(urls) > 0 {
		return urls[0]
	}
	return ""
}
