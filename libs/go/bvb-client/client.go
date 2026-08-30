package bvbclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// DefaultEndpoint is BVB's published financials service.
const DefaultEndpoint = "https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx"

// ErrInvalidArgument means the caller asked for something the service cannot be
// asked for, such as a window of zero days.
var ErrInvalidArgument = errors.New("invalid argument")

// Client reads companies and dividends from the BVB financials service.
//
// A Client is safe for concurrent use; the importer fetches several companies
// at once through one of them.
type Client struct {
	endpoint   string
	httpClient *http.Client
	log        *slog.Logger
	attempts   int
	backoff    time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithEndpoint overrides the service address, for tests or a mirror.
func WithEndpoint(endpoint string) Option {
	return func(c *Client) {
		if endpoint != "" {
			c.endpoint = endpoint
		}
	}
}

// WithHTTPClient supplies the HTTP client to call through.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithLogger supplies the logger the retry loop reports through.
func WithLogger(log *slog.Logger) Option {
	return func(c *Client) {
		if log != nil {
			c.log = log
		}
	}
}

// WithRetry sets how many times a failed call is retried and the base delay
// between attempts, which doubles each time.
func WithRetry(attempts int, backoff time.Duration) Option {
	return func(c *Client) {
		if attempts > 0 {
			c.attempts = attempts
		}
		if backoff > 0 {
			c.backoff = backoff
		}
	}
}

// New returns a Client reading from BVB.
func New(options ...Option) *Client {
	client := &Client{
		endpoint:   DefaultEndpoint,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		log:        slog.Default(),
		attempts:   3,
		backoff:    500 * time.Millisecond,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

// GetLastDividends returns one identification per dividend announced within the
// last days days.
//
// The result is BVB's own, unfiltered: a company that announced more than once
// in the window appears more than once, and a symbol may arrive in any casing.
func (c *Client) GetLastDividends(ctx context.Context, days int) ([]Identification, error) {
	if days <= 0 {
		return nil, fmt.Errorf("%w: days must be positive, got %d", ErrInvalidArgument, days)
	}

	var response getLastDividendsResponse
	err := c.retry(ctx, func() error {
		return c.call(ctx, "GetLastDividends", getLastDividends{NoDays: days}, &response)
	})
	if err != nil {
		return nil, fmt.Errorf("get companies announcing in the last %d days: %w", days, err)
	}
	return response.Result.Identifications, nil
}

// GetAvailableBalances returns every issuer that filed a balance of the given
// kind for the given year.
//
// It is the closest the service comes to enumerating the companies listed on
// BVB: there is no operation that returns them, and GetLastDividends only names
// the ones that announced a dividend recently. A year the filing season has not
// reached yet returns nothing rather than failing, so a caller after the whole
// market should ask for more than one.
func (c *Client) GetAvailableBalances(
	ctx context.Context,
	year int,
	reportType ReportType,
) ([]SymbolBalance, error) {
	if year <= 0 {
		return nil, fmt.Errorf("%w: year must be positive, got %d", ErrInvalidArgument, year)
	}
	if reportType == "" {
		return nil, fmt.Errorf("%w: report type is empty", ErrInvalidArgument)
	}

	request := getAvailableBalances{Year: year, ReportType: reportType}

	var response getAvailableBalancesResponse
	err := c.retry(ctx, func() error {
		return c.call(ctx, "GetAvailableBalances", request, &response)
	})
	if err != nil {
		return nil, fmt.Errorf("get the issuers filing %s balances for %d: %w", reportType, year, err)
	}

	return response.Result.Balances, nil
}

// GetDividends returns every dividend BVB has recorded for one ticker symbol.
func (c *Client) GetDividends(ctx context.Context, symbol string) ([]DividendInfo, error) {
	if symbol == "" {
		return nil, fmt.Errorf("%w: symbol is empty", ErrInvalidArgument)
	}

	request := getDividends{IdentityType: identityTypeSymbol, Identity: symbol}
	var response getDividendsResponse
	err := c.retry(ctx, func() error {
		return c.call(ctx, "GetDividends", request, &response)
	})
	if err != nil {
		return nil, fmt.Errorf("get dividends for %s: %w", symbol, err)
	}
	return response.Result.Infos, nil
}

// retry runs call until it succeeds, the attempts run out, or the context ends.
//
// A SOAP fault is the service saying it understood and refused, so it is not
// retried; transport failures are, since the import runs unattended and a
// single dropped connection should not cost a company its data for the day.
func (c *Client) retry(ctx context.Context, call func() error) error {
	var lastErr error
	delay := c.backoff

	for attempt := 1; attempt <= c.attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = call()
		if lastErr == nil {
			return nil
		}

		var fault *soapFault
		if errors.As(lastErr, &fault) {
			return lastErr
		}
		if attempt == c.attempts {
			break
		}

		c.log.Debug("retrying a failed BVB call",
			slog.Int("attempt", attempt),
			slog.Duration("delay", delay),
			slog.String("error", lastErr.Error()))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
	return lastErr
}
