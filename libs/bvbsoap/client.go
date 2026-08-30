package bvbsoap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// DefaultEndpoint is BVB's published financials service.
const DefaultEndpoint = "https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx"

// Client reads companies and dividends from the BVB financials service.
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

// WithLogger supplies the logger for values BVB reports in unexpected forms.
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

var _ domain.Source = (*Client)(nil)

// RecentlyAnnouncing returns the companies that announced a dividend within
// the last days days.
func (c *Client) RecentlyAnnouncing(ctx context.Context, days int) ([]domain.Company, error) {
	if days <= 0 {
		return nil, fmt.Errorf("%w: days must be positive, got %d", domain.ErrInvalidArgument, days)
	}

	var response getLastDividendsResponse
	err := c.retry(ctx, func() error {
		return c.call(ctx, "GetLastDividends", getLastDividends{NoDays: days}, &response)
	})
	if err != nil {
		return nil, fmt.Errorf("get companies announcing in the last %d days: %w", days, err)
	}
	return toCompanies(response.Result.Identifications, c.log), nil
}

// DividendsFor returns every dividend BVB has recorded for one company.
func (c *Client) DividendsFor(ctx context.Context, company domain.Company) ([]domain.Dividend, error) {
	if err := company.Validate(); err != nil {
		return nil, err
	}

	request := getDividends{
		IdentityType: identityTypeSymbol,
		Identity:     company.Symbol.String(),
	}
	var response getDividendsResponse
	err := c.retry(ctx, func() error {
		return c.call(ctx, "GetDividends", request, &response)
	})
	if err != nil {
		return nil, fmt.Errorf("get dividends for %s: %w", company.Symbol, err)
	}
	return toDividends(company, response.Result.Dividends.Infos, c.log), nil
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
