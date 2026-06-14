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
	if fuels[0].Perc != 45.2 {
		t.Errorf("fuels[0].Perc = %f, want 45.2", fuels[0].Perc)
	}
	if fuels[0].From == "" {
		t.Error("fuels[0].From should not be empty")
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

func TestGetFactors(t *testing.T) {
	payload := `{"data":[{"Biomass":120,"Coal":937,"Wind":0,"Nuclear":0}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	factors, err := c.GetFactors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(factors) != 4 {
		t.Fatalf("len(factors) = %d, want 4", len(factors))
	}
	// sorted alphabetically: Biomass, Coal, Nuclear, Wind
	if factors[0].Fuel != "Biomass" {
		t.Errorf("factors[0].Fuel = %q, want Biomass", factors[0].Fuel)
	}
	if factors[0].Factor != 120 {
		t.Errorf("factors[0].Factor = %d, want 120", factors[0].Factor)
	}
	if factors[1].Fuel != "Coal" {
		t.Errorf("factors[1].Fuel = %q, want Coal", factors[1].Fuel)
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
