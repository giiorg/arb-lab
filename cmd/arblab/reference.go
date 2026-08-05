package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/platform/config"
	"github.com/giiorg/arb-lab/internal/strategy/infrastructure/driven"
)

var errReferenceEndpoint = errors.New("arblab reference fetch: no public endpoint for row")

type referenceClientBuilder func(driven.Document) (map[driven.FetchCell]driven.FetchClient, error)

// referenceFetch writes a new fetched-provenance revision. The output is a new
// identity: exclusive creation is the symlink- and spelling-independent guard
// against overwriting the calibration-pinned input or any earlier revision.
func referenceFetch(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reference fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	referencePath := fs.String("reference", "", "input reference document (required)")
	outPath := fs.String("out", "", "new fetched reference document (required, must not exist)")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *referencePath == "" || *outPath == "" {
		fmt.Fprintln(stderr, "reference fetch: -reference and -out are required")
		return 64
	}

	wall, err := clock.NewWall(nil)
	if err != nil {
		fmt.Fprintln(stderr, "reference fetch:", err)
		return 70
	}
	if err := fetchReferenceFile(context.Background(), *referencePath, *outPath, wall, buildReferenceClients); err != nil {
		fmt.Fprintln(stderr, "reference fetch:", err)
		return 70
	}
	fmt.Fprintln(stdout, "reference fetch:", *outPath)
	return 0
}

// fetchReferenceFile keeps file creation after every fetch, bind, validation,
// and marshal. A failure before that point has no filesystem state to unwind;
// a failure after exclusive creation removes only the file this call created.
func fetchReferenceFile(ctx context.Context, referencePath, outPath string, clk clock.Clock, build referenceClientBuilder) error {
	data, err := os.ReadFile(referencePath)
	if err != nil {
		return fmt.Errorf("reference %s: %w", referencePath, err)
	}
	store, err := config.New[driven.Document](referencePath, clk.Now(), data)
	if err != nil {
		return fmt.Errorf("reference %s: %w", referencePath, err)
	}
	doc := store.Current().Value
	clients, err := build(doc)
	if err != nil {
		return err
	}
	fetched, err := driven.FetchLots(ctx, clients, doc, clk)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(fetched, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal fetched reference: %w", err)
	}
	body = append(body, '\n')
	if err := writeReferenceExclusive(outPath, body); err != nil {
		return fmt.Errorf("output %s: %w", outPath, err)
	}
	return nil
}

func buildReferenceClients(doc driven.Document) (map[driven.FetchCell]driven.FetchClient, error) {
	clients := make(map[driven.FetchCell]driven.FetchClient, len(doc.Lots))
	for i, row := range doc.Lots {
		cell := driven.FetchCell{Venue: row.Venue, Product: row.Product}
		cfg, ok := referencePublicConfig(row)
		if !ok {
			return nil, fmt.Errorf("lots[%d] venue %q product %q: %w", i, row.Venue, row.Product, errReferenceEndpoint)
		}
		client, err := transport.NewPublicClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("lots[%d] venue %q product %q endpoint: %v: %w", i, row.Venue, row.Product, err, errReferenceEndpoint)
		}
		if prior, exists := clients[cell]; exists && prior.URL() != client.URL() {
			return nil, fmt.Errorf("lots[%d] venue %q product %q needs both %q and %q: %w",
				i, row.Venue, row.Product, prior.URL(), client.URL(), errReferenceEndpoint)
		}
		clients[cell] = client
	}
	return clients, nil
}

func referencePublicConfig(row driven.LotRow) (transport.PublicConfig, bool) {
	cfg := transport.PublicConfig{Scheme: "https", Timeout: 10 * time.Second, ReadLimit: 4 << 20}
	switch (driven.FetchCell{Venue: row.Venue, Product: row.Product}) {
	case driven.FetchCell{Venue: "binance", Product: "spot"}:
		cfg.Host = "api.binance.com"
		cfg.Path = "/api/v3/exchangeInfo"
		cfg.Params = transport.Params{{Key: "symbol", Value: row.Symbol}}
	case driven.FetchCell{Venue: "binance", Product: "linear_perp"}:
		cfg.Host = "fapi.binance.com"
		cfg.Path = "/fapi/v1/exchangeInfo"
	case driven.FetchCell{Venue: "bybit", Product: "spot"}:
		cfg.Host = "api.bybit.com"
		cfg.Path = "/v5/market/instruments-info"
		cfg.Params = transport.Params{{Key: "category", Value: "spot"}, {Key: "symbol", Value: row.Symbol}}
	case driven.FetchCell{Venue: "bybit", Product: "linear_perp"}:
		cfg.Host = "api.bybit.com"
		cfg.Path = "/v5/market/instruments-info"
		cfg.Params = transport.Params{{Key: "category", Value: "linear"}, {Key: "symbol", Value: row.Symbol}}
	default:
		return transport.PublicConfig{}, false
	}
	return cfg, true
}

func writeReferenceExclusive(path string, body []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = f.Close()
			_ = os.Remove(path)
		}
	}()
	if _, err := f.Write(body); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}
