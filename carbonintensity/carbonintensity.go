// Package carbonintensity is the library behind the carbonintensity command line:
// the HTTP client, request shaping, and the typed data models for the UK Carbon
// Intensity API (api.carbonintensity.org.uk). No API key required.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
package carbonintensity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// DefaultUserAgent identifies the client to the Carbon Intensity API.
const DefaultUserAgent = "carbonintensity-cli/dev (+https://github.com/tamnd/carbonintensity-cli)"

// Host is the API host this client talks to.
const Host = "api.carbonintensity.org.uk"

// Config holds runtime configuration for the client.
type Config struct {
	BaseURL string
	Rate    time.Duration
	Retries int
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults for the Carbon Intensity API.
func DefaultConfig() Config {
	return Config{
		BaseURL: "https://api.carbonintensity.org.uk",
		Rate:    200 * time.Millisecond,
		Retries: 3,
		Timeout: 15 * time.Second,
	}
}

// Client talks to the Carbon Intensity API over HTTP.
type Client struct {
	http      *http.Client
	userAgent string
	baseURL   string
	rate      time.Duration
	retries   int
	last      time.Time
}

// NewClient returns a Client using DefaultConfig.
func NewClient() *Client {
	return NewClientWithConfig(DefaultConfig())
}

// NewClientWithConfig returns a Client configured with cfg.
func NewClientWithConfig(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	retries := cfg.Retries
	if retries <= 0 {
		retries = 3
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.carbonintensity.org.uk"
	}
	return &Client{
		http:      &http.Client{Timeout: timeout},
		userAgent: DefaultUserAgent,
		baseURL:   baseURL,
		rate:      cfg.Rate,
		retries:   retries,
	}
}

// get fetches the given path (e.g. "/intensity") and unmarshals JSON into dst.
func (c *Client) get(ctx context.Context, path string, dst any) error {
	url := c.baseURL + path
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			if jerr := json.Unmarshal(body, dst); jerr != nil {
				return fmt.Errorf("decode %s: %w", path, jerr)
			}
			return nil
		}
		lastErr = err
		if !retry {
			return fmt.Errorf("get %s: %w", path, err)
		}
	}
	return fmt.Errorf("get %s: %w", path, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least rate has passed since the previous request.
func (c *Client) pace() {
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- output types ---

// Intensity is the current carbon intensity for Great Britain.
type Intensity struct {
	From     string `kit:"id" json:"from"`
	To       string `json:"to"`
	Forecast int    `json:"forecast"`
	Actual   int    `json:"actual"`
	Index    string `json:"index"`
}

// GenerationFuel is one fuel's share of the current electricity generation mix.
type GenerationFuel struct {
	Fuel    string `kit:"id" json:"fuel"`
	Percent string `json:"percent"` // formatted as "%.1f"
}

// RegionalIntensity is the carbon intensity for one UK distribution network region.
type RegionalIntensity struct {
	RegionID  int    `kit:"id" json:"region_id"`
	ShortName string `json:"short_name"`
	DNORegion string `json:"dno_region"`
	From      string `json:"from"`
	Forecast  int    `json:"forecast"`
	Index     string `json:"index"`
}


// --- wire types ---

type wireIntensityResponse struct {
	Data []struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Intensity struct {
			Forecast int    `json:"forecast"`
			Actual   int    `json:"actual"`
			Index    string `json:"index"`
		} `json:"intensity"`
	} `json:"data"`
}

type wireGenerationResponse struct {
	Data struct {
		From          string `json:"from"`
		To            string `json:"to"`
		GenerationMix []struct {
			Fuel string  `json:"fuel"`
			Perc float64 `json:"perc"`
		} `json:"generationmix"`
	} `json:"data"`
}

type wireRegionalResponse struct {
	Data []struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Regions []struct {
			RegionID  int    `json:"regionid"`
			DNORegion string `json:"dnoregion"`
			ShortName string `json:"shortname"`
			Intensity struct {
				Forecast int    `json:"forecast"`
				Index    string `json:"index"`
			} `json:"intensity"`
		} `json:"regions"`
	} `json:"data"`
}


// --- client methods ---

// GetIntensity returns the current national carbon intensity.
func (c *Client) GetIntensity(ctx context.Context) (*Intensity, error) {
	var w wireIntensityResponse
	if err := c.get(ctx, "/intensity", &w); err != nil {
		return nil, err
	}
	if len(w.Data) == 0 {
		return nil, fmt.Errorf("intensity: empty response")
	}
	d := w.Data[0]
	return &Intensity{
		From:     d.From,
		To:       d.To,
		Forecast: d.Intensity.Forecast,
		Actual:   d.Intensity.Actual,
		Index:    d.Intensity.Index,
	}, nil
}

// GetGeneration returns the current generation mix as one record per fuel type.
func (c *Client) GetGeneration(ctx context.Context) ([]GenerationFuel, error) {
	var w wireGenerationResponse
	if err := c.get(ctx, "/generation", &w); err != nil {
		return nil, err
	}
	fuels := make([]GenerationFuel, 0, len(w.Data.GenerationMix))
	for _, m := range w.Data.GenerationMix {
		fuels = append(fuels, GenerationFuel{
			Fuel:    m.Fuel,
			Percent: fmt.Sprintf("%.1f", m.Perc),
		})
	}
	return fuels, nil
}

// GetRegional returns the current carbon intensity for each of the 18 UK
// distribution network regions.
func (c *Client) GetRegional(ctx context.Context) ([]RegionalIntensity, error) {
	var w wireRegionalResponse
	if err := c.get(ctx, "/regional", &w); err != nil {
		return nil, err
	}
	if len(w.Data) == 0 {
		return nil, fmt.Errorf("regional: empty response")
	}
	d := w.Data[0]
	regions := make([]RegionalIntensity, 0, len(d.Regions))
	for _, r := range d.Regions {
		regions = append(regions, RegionalIntensity{
			RegionID:  r.RegionID,
			ShortName: r.ShortName,
			DNORegion: r.DNORegion,
			From:      d.From,
			Forecast:  r.Intensity.Forecast,
			Index:     r.Intensity.Index,
		})
	}
	return regions, nil
}

// GetHistory returns carbon intensity records for the given date range.
// from and to must be ISO 8601 datetime strings, e.g. "2025-01-01T00:00Z".
func (c *Client) GetHistory(ctx context.Context, from, to string) ([]Intensity, error) {
	path := "/intensity/" + url.PathEscape(from) + "/" + url.PathEscape(to)
	var w wireIntensityResponse
	if err := c.get(ctx, path, &w); err != nil {
		return nil, err
	}
	out := make([]Intensity, 0, len(w.Data))
	for _, d := range w.Data {
		out = append(out, Intensity{
			From:     d.From,
			To:       d.To,
			Forecast: d.Intensity.Forecast,
			Actual:   d.Intensity.Actual,
			Index:    d.Intensity.Index,
		})
	}
	return out, nil
}
