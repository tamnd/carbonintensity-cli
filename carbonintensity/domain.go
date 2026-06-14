package carbonintensity

import (
	"context"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes the UK Carbon Intensity API as a kit Domain: a driver that
// a multi-domain host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/carbonintensity-cli/carbonintensity"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// carbonintensity:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone carbonintensity binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the carbonintensity driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "carbonintensity",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "carbonintensity",
			Short:  "A command line for the UK Carbon Intensity API.",
			Long: `A command line for the UK Carbon Intensity API.

carbonintensity reads live electricity data from api.carbonintensity.org.uk,
shapes it into clean records, and prints output that pipes into the rest of
your tools. No API key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/carbonintensity-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name: "intensity", Group: "read", Single: true,
		Summary: "Current national carbon intensity for Great Britain",
		URIType: "query", Resolver: true,
	}, getIntensity)

	kit.Handle(app, kit.OpMeta{
		Name: "generation", Group: "read",
		Summary: "Current electricity generation mix by fuel type",
		URIType: "query",
	}, getGeneration)

	kit.Handle(app, kit.OpMeta{
		Name: "regional", Group: "read",
		Summary: "Current carbon intensity for all 18 UK regions",
		URIType: "query",
	}, getRegional)

	kit.Handle(app, kit.OpMeta{
		Name:    "history",
		Group:   "read",
		Summary: "Carbon intensity for a date range (ISO datetimes, e.g. 2025-01-01T00:00Z)",
		URIType: "query",
		Args: []kit.Arg{
			{Name: "from", Help: "start datetime, e.g. 2025-01-01T00:00Z"},
			{Name: "to", Help: "end datetime, e.g. 2025-01-01T02:00Z"},
		},
	}, getHistory)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	dcfg := DefaultConfig()
	if cfg.Rate > 0 {
		dcfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		dcfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		dcfg.Timeout = cfg.Timeout
	}
	c := NewClientWithConfig(dcfg)
	if cfg.UserAgent != "" {
		c.userAgent = cfg.UserAgent
	}
	return c, nil
}

// --- inputs (inject-only, no user args needed for these endpoints) ---

type intensityInput struct {
	Client *Client `kit:"inject"`
}

type generationInput struct {
	Client *Client `kit:"inject"`
}

type regionalInput struct {
	Client *Client `kit:"inject"`
}

type historyInput struct {
	Client *Client `kit:"inject"`
	From   string  `kit:"arg" json:"from"`
	To     string  `kit:"arg" json:"to"`
}

// --- handlers ---

func getIntensity(ctx context.Context, in intensityInput, emit func(*Intensity) error) error {
	v, err := in.Client.GetIntensity(ctx)
	if err != nil {
		return err
	}
	return emit(v)
}

func getGeneration(ctx context.Context, in generationInput, emit func(GenerationFuel) error) error {
	fuels, err := in.Client.GetGeneration(ctx)
	if err != nil {
		return err
	}
	for _, f := range fuels {
		if err := emit(f); err != nil {
			return err
		}
	}
	return nil
}

func getRegional(ctx context.Context, in regionalInput, emit func(RegionalIntensity) error) error {
	regions, err := in.Client.GetRegional(ctx)
	if err != nil {
		return err
	}
	for _, r := range regions {
		if err := emit(r); err != nil {
			return err
		}
	}
	return nil
}

func getHistory(ctx context.Context, in historyInput, emit func(*Intensity) error) error {
	records, err := in.Client.GetHistory(ctx, in.From, in.To)
	if err != nil {
		return err
	}
	for i := range records {
		if err := emit(&records[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns any input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	if input == "" {
		return "", "", errs.Usage("carbonintensity: empty input")
	}
	return "query", input, nil
}

// Locate returns the live URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "query" {
		return "", errs.Usage("carbonintensity has no resource type %q", uriType)
	}
	return "https://" + Host + "/intensity", nil
}
