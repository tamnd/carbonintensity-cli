package carbonintensity_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tamnd/carbonintensity-cli/carbonintensity"
)

func newTestClient(srv *httptest.Server) *carbonintensity.Client {
	cfg := carbonintensity.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Timeout = 5 * time.Second
	return carbonintensity.NewClientWithConfig(cfg)
}

func TestGetIntensity(t *testing.T) {
	payload := `{"data":[{"from":"2026-06-14T16:30Z","to":"2026-06-14T16:45Z","intensity":{"forecast":101,"actual":95,"index":"moderate"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	v, err := c.GetIntensity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Forecast != 101 {
		t.Errorf("Forecast = %d, want 101", v.Forecast)
	}
	if v.Actual != 95 {
		t.Errorf("Actual = %d, want 95", v.Actual)
	}
	if v.Index != "moderate" {
		t.Errorf("Index = %q, want moderate", v.Index)
	}
}

func TestGetGeneration(t *testing.T) {
	payload := `{"data":{"from":"2026-06-14T16:30Z","to":"2026-06-14T16:45Z","generationmix":[{"fuel":"gas","perc":45.2},{"fuel":"wind","perc":25.1}]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	fuels, err := c.GetGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(fuels) != 2 {
		t.Fatalf("len(fuels) = %d, want 2", len(fuels))
	}
	if fuels[0].Fuel != "gas" {
		t.Errorf("fuels[0].Fuel = %q, want gas", fuels[0].Fuel)
	}
	if fuels[0].Percent != "45.2" {
		t.Errorf("fuels[0].Percent = %q, want 45.2", fuels[0].Percent)
	}
	if fuels[1].Percent != "25.1" {
		t.Errorf("fuels[1].Percent = %q, want 25.1", fuels[1].Percent)
	}
}

func TestGetRegional(t *testing.T) {
	payload := `{"data":[{"from":"2026-06-14T16:30Z","to":"2026-06-14T16:45Z","regions":[{"regionid":1,"dnoregion":"Scottish Hydro Electric Power Distribution","shortname":"North Scotland","intensity":{"forecast":0,"index":"very low"},"generationmix":[]}]}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	regions, err := c.GetRegional(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(regions) != 1 {
		t.Fatalf("len(regions) = %d, want 1", len(regions))
	}
	if regions[0].RegionID != 1 {
		t.Errorf("RegionID = %d, want 1", regions[0].RegionID)
	}
	if regions[0].ShortName != "North Scotland" {
		t.Errorf("ShortName = %q, want North Scotland", regions[0].ShortName)
	}
	if regions[0].Index != "very low" {
		t.Errorf("Index = %q, want very low", regions[0].Index)
	}
}

func TestGetHistory(t *testing.T) {
	payload := `{"data":[{"from":"2024-12-31T23:30Z","to":"2025-01-01T00:00Z","intensity":{"forecast":53,"actual":51,"index":"low"}},{"from":"2025-01-01T00:00Z","to":"2025-01-01T00:30Z","intensity":{"forecast":55,"actual":52,"index":"low"}}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	records, err := c.GetHistory(context.Background(), "2025-01-01T00:00Z", "2025-01-01T02:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Forecast != 53 {
		t.Errorf("records[0].Forecast = %d, want 53", records[0].Forecast)
	}
	if records[0].Actual != 51 {
		t.Errorf("records[0].Actual = %d, want 51", records[0].Actual)
	}
	if records[0].Index != "low" {
		t.Errorf("records[0].Index = %q, want low", records[0].Index)
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"from":"2026-06-14T16:30Z","to":"2026-06-14T16:45Z","intensity":{"forecast":100,"actual":90,"index":"low"}}]}`))
	}))
	defer srv.Close()

	cfg := carbonintensity.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	cfg.Timeout = 10 * time.Second
	c := carbonintensity.NewClientWithConfig(cfg)

	start := time.Now()
	v, err := c.GetIntensity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Forecast != 100 {
		t.Errorf("Forecast = %d after retries, want 100", v.Forecast)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := carbonintensity.DefaultConfig()
	if cfg.BaseURL == "" {
		t.Error("DefaultConfig BaseURL is empty")
	}
	if cfg.Rate <= 0 {
		t.Error("DefaultConfig Rate should be positive")
	}
	if cfg.Retries <= 0 {
		t.Error("DefaultConfig Retries should be positive")
	}
	if cfg.Timeout <= 0 {
		t.Error("DefaultConfig Timeout should be positive")
	}
}

func TestIntensityJSONShape(t *testing.T) {
	v := carbonintensity.Intensity{
		From:     "2026-06-14T16:30Z",
		To:       "2026-06-14T16:45Z",
		Forecast: 101,
		Actual:   95,
		Index:    "moderate",
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"from", "to", "forecast", "actual", "index"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}
}

func TestGenerationFuelJSONShape(t *testing.T) {
	f := carbonintensity.GenerationFuel{
		Fuel:    "wind",
		Percent: "20.0",
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"fuel", "percent"} {
		if _, ok := m[key]; !ok {
			t.Errorf("JSON missing key %q", key)
		}
	}
}
