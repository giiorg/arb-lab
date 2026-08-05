package driven

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/giiorg/arb-lab/internal/engine/venue/transport"
	"github.com/giiorg/arb-lab/internal/platform/clock"
	"github.com/giiorg/arb-lab/internal/strategy/domain"
)

var (
	// ErrFetchStatus reports an HTTP or venue-envelope status that cannot
	// support a fetched-provenance claim.
	ErrFetchStatus = fmt.Errorf("strategy/driven: reference fetch status refused")

	// ErrFetchSymbolAbsent reports a response whose roster does not contain
	// the exact requested symbol. Binance fapi makes this selection local by
	// ignoring its symbol query parameter.
	ErrFetchSymbolAbsent = fmt.Errorf("strategy/driven: reference fetch symbol absent")

	// ErrFetchFilterAbsent reports an absent filter object or empty decimal
	// member. Empty venue words must not become permissive zeroes.
	ErrFetchFilterAbsent = fmt.Errorf("strategy/driven: reference fetch filter absent")

	// ErrFetchIdentityMismatch reports a response whose base, quote, or
	// contract identity is not the cell the input row names.
	ErrFetchIdentityMismatch = fmt.Errorf("strategy/driven: reference fetch identity mismatch")

	// ErrFetchNotTrading reports a symbol that still publishes valid numeric
	// filters but is not currently tradeable.
	ErrFetchNotTrading = fmt.Errorf("strategy/driven: reference fetch symbol not trading")

	// ErrFetchDecode reports bytes that are not the endpoint's JSON shape.
	ErrFetchDecode = fmt.Errorf("strategy/driven: reference fetch response invalid")
)

// FetchCell is the client-routing key shared by a reference document and the
// composition root. Symbol is deliberately absent: §69's document has one lot
// row per venue/product cell, while the symbol belongs in the endpoint URL.
type FetchCell struct {
	Venue   string
	Product string
}

// FetchClient is the unsigned public-read surface FetchLots needs. Keeping the
// response and URL together prevents the provenance source from drifting from
// the endpoint that supplied the bytes.
type FetchClient interface {
	Fetch(context.Context) (transport.PollResult, error)
	URL() string
}

var _ FetchClient = (*transport.PublicClient)(nil)

// fetchedLots carries venue words verbatim until Document.Validate converts
// them through the domain's decimal authority.
type fetchedLots struct {
	step         string
	minQty       string
	minNotional  string
	tick         string
	base         string
	quote        string
	status       string
	contractType string
}

type binanceLotFilter struct {
	FilterType  string `json:"filterType"`
	StepSize    string `json:"stepSize"`
	MinQty      string `json:"minQty"`
	TickSize    string `json:"tickSize"`
	MinNotional string `json:"minNotional"`
	Notional    string `json:"notional"`
}

type binanceLotSymbol struct {
	Symbol       string             `json:"symbol"`
	BaseAsset    string             `json:"baseAsset"`
	QuoteAsset   string             `json:"quoteAsset"`
	Status       string             `json:"status"`
	ContractType string             `json:"contractType"`
	Filters      []binanceLotFilter `json:"filters"`
}

func decodeBinanceSpotLots(body []byte, symbol string) (fetchedLots, error) {
	s, err := decodeBinanceSymbol(body, symbol, "binance spot")
	if err != nil {
		return fetchedLots{}, err
	}
	lot, err := requireBinanceFilter(s.Symbol, s.Filters, "LOT_SIZE")
	if err != nil {
		return fetchedLots{}, err
	}
	price, err := requireBinanceFilter(s.Symbol, s.Filters, "PRICE_FILTER")
	if err != nil {
		return fetchedLots{}, err
	}
	notional, err := requireBinanceFilter(s.Symbol, s.Filters, "NOTIONAL")
	if err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "LOT_SIZE.stepSize", lot.StepSize); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "LOT_SIZE.minQty", lot.MinQty); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "PRICE_FILTER.tickSize", price.TickSize); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "NOTIONAL.minNotional", notional.MinNotional); err != nil {
		return fetchedLots{}, err
	}
	if err := requireBinanceIdentity(s, false); err != nil {
		return fetchedLots{}, err
	}
	return fetchedLots{
		step: lot.StepSize, minQty: lot.MinQty, minNotional: notional.MinNotional,
		tick: price.TickSize, base: s.BaseAsset, quote: s.QuoteAsset, status: s.Status,
	}, nil
}

func decodeBinancePerpLots(body []byte, symbol string) (fetchedLots, error) {
	s, err := decodeBinanceSymbol(body, symbol, "binance fapi")
	if err != nil {
		return fetchedLots{}, err
	}
	lot, err := requireBinanceFilter(s.Symbol, s.Filters, "LOT_SIZE")
	if err != nil {
		return fetchedLots{}, err
	}
	price, err := requireBinanceFilter(s.Symbol, s.Filters, "PRICE_FILTER")
	if err != nil {
		return fetchedLots{}, err
	}
	notional, err := requireBinanceFilter(s.Symbol, s.Filters, "MIN_NOTIONAL")
	if err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "LOT_SIZE.stepSize", lot.StepSize); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "LOT_SIZE.minQty", lot.MinQty); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "PRICE_FILTER.tickSize", price.TickSize); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "MIN_NOTIONAL.notional", notional.Notional); err != nil {
		return fetchedLots{}, err
	}
	if err := requireBinanceIdentity(s, true); err != nil {
		return fetchedLots{}, err
	}
	return fetchedLots{
		step: lot.StepSize, minQty: lot.MinQty, minNotional: notional.Notional,
		tick: price.TickSize, base: s.BaseAsset, quote: s.QuoteAsset, status: s.Status,
		contractType: s.ContractType,
	}, nil
}

func decodeBinanceSymbol(body []byte, symbol, endpoint string) (binanceLotSymbol, error) {
	var info struct {
		Symbols []binanceLotSymbol `json:"symbols"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return binanceLotSymbol{}, fmt.Errorf("%s response: %v: %w", endpoint, err, ErrFetchDecode)
	}
	for _, candidate := range info.Symbols {
		if candidate.Symbol == symbol {
			return candidate, nil
		}
	}
	return binanceLotSymbol{}, fmt.Errorf("%s symbol %q: %w", endpoint, symbol, ErrFetchSymbolAbsent)
}

func requireBinanceFilter(symbol string, filters []binanceLotFilter, filterType string) (binanceLotFilter, error) {
	for _, filter := range filters {
		if filter.FilterType == filterType {
			return filter, nil
		}
	}
	return binanceLotFilter{}, fmt.Errorf("symbol %q filter %s: %w", symbol, filterType, ErrFetchFilterAbsent)
}

func requireBinanceIdentity(s binanceLotSymbol, contractRequired bool) error {
	if s.BaseAsset == "" {
		return fmt.Errorf("symbol %q baseAsset: %w", s.Symbol, ErrFetchIdentityMismatch)
	}
	if s.QuoteAsset == "" {
		return fmt.Errorf("symbol %q quoteAsset: %w", s.Symbol, ErrFetchIdentityMismatch)
	}
	if s.Status == "" {
		return fmt.Errorf("symbol %q status: %w", s.Symbol, ErrFetchNotTrading)
	}
	if contractRequired && s.ContractType == "" {
		return fmt.Errorf("symbol %q contractType: %w", s.Symbol, ErrFetchIdentityMismatch)
	}
	return nil
}

type bybitLotSymbol struct {
	Symbol        string `json:"symbol"`
	BaseCoin      string `json:"baseCoin"`
	QuoteCoin     string `json:"quoteCoin"`
	Status        string `json:"status"`
	ContractType  string `json:"contractType"`
	LotSizeFilter struct {
		BasePrecision    string `json:"basePrecision"`
		MinOrderQty      string `json:"minOrderQty"`
		MinOrderAmt      string `json:"minOrderAmt"`
		QtyStep          string `json:"qtyStep"`
		MinNotionalValue string `json:"minNotionalValue"`
	} `json:"lotSizeFilter"`
	PriceFilter struct {
		TickSize string `json:"tickSize"`
	} `json:"priceFilter"`
}

func decodeBybitSpotLots(body []byte, symbol string) (fetchedLots, error) {
	s, err := decodeBybitSymbol(body, symbol, "bybit spot")
	if err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "lotSizeFilter.basePrecision", s.LotSizeFilter.BasePrecision); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "lotSizeFilter.minOrderQty", s.LotSizeFilter.MinOrderQty); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "lotSizeFilter.minOrderAmt", s.LotSizeFilter.MinOrderAmt); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "priceFilter.tickSize", s.PriceFilter.TickSize); err != nil {
		return fetchedLots{}, err
	}
	if err := requireBybitIdentity(s, false); err != nil {
		return fetchedLots{}, err
	}
	return fetchedLots{
		step: s.LotSizeFilter.BasePrecision, minQty: s.LotSizeFilter.MinOrderQty,
		minNotional: s.LotSizeFilter.MinOrderAmt, tick: s.PriceFilter.TickSize,
		base: s.BaseCoin, quote: s.QuoteCoin, status: s.Status,
	}, nil
}

func decodeBybitLinearLots(body []byte, symbol string) (fetchedLots, error) {
	s, err := decodeBybitSymbol(body, symbol, "bybit linear")
	if err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "lotSizeFilter.qtyStep", s.LotSizeFilter.QtyStep); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "lotSizeFilter.minOrderQty", s.LotSizeFilter.MinOrderQty); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "lotSizeFilter.minNotionalValue", s.LotSizeFilter.MinNotionalValue); err != nil {
		return fetchedLots{}, err
	}
	if err := requireFetchField(s.Symbol, "priceFilter.tickSize", s.PriceFilter.TickSize); err != nil {
		return fetchedLots{}, err
	}
	if err := requireBybitIdentity(s, true); err != nil {
		return fetchedLots{}, err
	}
	return fetchedLots{
		step: s.LotSizeFilter.QtyStep, minQty: s.LotSizeFilter.MinOrderQty,
		minNotional: s.LotSizeFilter.MinNotionalValue, tick: s.PriceFilter.TickSize,
		base: s.BaseCoin, quote: s.QuoteCoin, status: s.Status, contractType: s.ContractType,
	}, nil
}

func decodeBybitSymbol(body []byte, symbol, endpoint string) (bybitLotSymbol, error) {
	var env struct {
		RetCode *int   `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []bybitLotSymbol `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return bybitLotSymbol{}, fmt.Errorf("%s response: %v: %w", endpoint, err, ErrFetchDecode)
	}
	if env.RetCode == nil {
		return bybitLotSymbol{}, fmt.Errorf("%s retCode missing: %w", endpoint, ErrFetchStatus)
	}
	if *env.RetCode != 0 {
		return bybitLotSymbol{}, fmt.Errorf("%s retCode %d %q: %w", endpoint, *env.RetCode, env.RetMsg, ErrFetchStatus)
	}
	for _, candidate := range env.Result.List {
		if candidate.Symbol == symbol {
			return candidate, nil
		}
	}
	return bybitLotSymbol{}, fmt.Errorf("%s symbol %q: %w", endpoint, symbol, ErrFetchSymbolAbsent)
}

func requireBybitIdentity(s bybitLotSymbol, contractRequired bool) error {
	if s.BaseCoin == "" {
		return fmt.Errorf("symbol %q baseCoin: %w", s.Symbol, ErrFetchIdentityMismatch)
	}
	if s.QuoteCoin == "" {
		return fmt.Errorf("symbol %q quoteCoin: %w", s.Symbol, ErrFetchIdentityMismatch)
	}
	if s.Status == "" {
		return fmt.Errorf("symbol %q status: %w", s.Symbol, ErrFetchNotTrading)
	}
	if contractRequired && s.ContractType == "" {
		return fmt.Errorf("symbol %q contractType: %w", s.Symbol, ErrFetchIdentityMismatch)
	}
	return nil
}

func requireFetchField(symbol, field, value string) error {
	if value == "" {
		return fmt.Errorf("symbol %q field %s: %w", symbol, field, ErrFetchFilterAbsent)
	}
	return nil
}

type fetchCellSpec struct {
	decode            func([]byte, string) (fetchedLots, error)
	tradingStatus     string
	venueContractType string
}

func fetchSpec(cell FetchCell) (fetchCellSpec, bool) {
	switch cell {
	case FetchCell{Venue: "binance", Product: "spot"}:
		return fetchCellSpec{decode: decodeBinanceSpotLots, tradingStatus: "TRADING"}, true
	case FetchCell{Venue: "binance", Product: "linear_perp"}:
		return fetchCellSpec{decode: decodeBinancePerpLots, tradingStatus: "TRADING", venueContractType: "PERPETUAL"}, true
	case FetchCell{Venue: "bybit", Product: "spot"}:
		return fetchCellSpec{decode: decodeBybitSpotLots, tradingStatus: "Trading"}, true
	case FetchCell{Venue: "bybit", Product: "linear_perp"}:
		return fetchCellSpec{decode: decodeBybitLinearLots, tradingStatus: "Trading", venueContractType: "LinearPerpetual"}, true
	}
	return fetchCellSpec{}, false
}

// FetchLots replaces every lot row only after every venue read, identity bind,
// and document validation succeeds. On failure the zero Document is returned,
// so a caller cannot accidentally persist an assembled prefix.
func FetchLots(ctx context.Context, clients map[FetchCell]FetchClient, doc Document, clk clock.Clock) (Document, error) {
	if err := doc.Validate(); err != nil {
		return Document{}, fmt.Errorf("reference input: %w", err)
	}

	out := doc
	out.Lots = make([]LotRow, len(doc.Lots))
	for i, row := range doc.Lots {
		cell := FetchCell{Venue: row.Venue, Product: row.Product}
		spec, ok := fetchSpec(cell)
		if !ok {
			return Document{}, fmt.Errorf("lots[%d] venue %q product %q: %w", i, row.Venue, row.Product, ErrFetchIdentityMismatch)
		}
		client, ok := clients[cell]
		if !ok || client == nil {
			return Document{}, fmt.Errorf("lots[%d] venue %q product %q client: %w", i, row.Venue, row.Product, ErrFetchIdentityMismatch)
		}
		result, err := client.Fetch(ctx)
		if err != nil {
			return Document{}, fmt.Errorf("lots[%d] %s/%s fetch: %w", i, row.Venue, row.Product, err)
		}
		if result.Status != http.StatusOK {
			return Document{}, fmt.Errorf("lots[%d] %s/%s HTTP status %d: %w", i, row.Venue, row.Product, result.Status, ErrFetchStatus)
		}
		fetched, err := spec.decode(result.Body, row.Symbol)
		if err != nil {
			return Document{}, fmt.Errorf("lots[%d] %s/%s symbol %q: %w", i, row.Venue, row.Product, row.Symbol, err)
		}
		if fetched.base != row.Base {
			return Document{}, fmt.Errorf("lots[%d] base row %q venue %q: %w", i, row.Base, fetched.base, ErrFetchIdentityMismatch)
		}
		if fetched.quote != row.Quote {
			return Document{}, fmt.Errorf("lots[%d] quote row %q venue %q: %w", i, row.Quote, fetched.quote, ErrFetchIdentityMismatch)
		}
		if fetched.status != spec.tradingStatus {
			return Document{}, fmt.Errorf("lots[%d] status %q, want %q: %w", i, fetched.status, spec.tradingStatus, ErrFetchNotTrading)
		}
		if fetched.contractType != spec.venueContractType {
			return Document{}, fmt.Errorf("lots[%d] contractType %q, want %q: %w", i, fetched.contractType, spec.venueContractType, ErrFetchIdentityMismatch)
		}
		out.Lots[i] = LotRow{
			Venue: row.Venue, Product: row.Product, Contract: row.Contract, Symbol: row.Symbol,
			Base: row.Base, Step: fetched.step, MinQty: fetched.minQty,
			Quote: row.Quote, MinNotional: fetched.minNotional, Tick: fetched.tick,
			Class: domain.ProvenanceFetched.String(), Source: client.URL(),
		}
	}

	observedAt := clk.Now().UTC().Format(time.RFC3339)
	for i := range out.Lots {
		out.Lots[i].ObservedAt = observedAt
	}
	if err := out.Validate(); err != nil {
		return Document{}, fmt.Errorf("fetched reference document: %w", err)
	}
	return out, nil
}
