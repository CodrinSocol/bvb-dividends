package apiserver_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/apiserver"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// newHandler builds the API over repositories holding the given fixtures.
func newHandler(t *testing.T, companies []domain.Company, dividends []domain.Dividend) http.Handler {
	t.Helper()

	server, err := apiserver.NewServer(
		&stubCompanies{items: companies},
		&stubDividends{items: dividends},
	)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	handler, err := apiserver.NewHandler(context.Background(), server, apiserver.Options{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		AllowedOrigins: []string{"http://localhost:4200"},
		RequestTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return handler
}

// get performs a request and returns the status and decoded JSON body.
func get(t *testing.T, handler http.Handler, path string, query url.Values) (int, map[string]any) {
	t.Helper()

	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))

	var body map[string]any
	if recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v\nbody: %s", target, err, recorder.Body.String())
		}
	}
	return recorder.Code, body
}

func fixtureDividend(company domain.Symbol, year int, exDate string) domain.Dividend {
	d := domain.Dividend{Company: company, Year: year, Type: "cash"}
	if exDate != "" {
		parsed, err := domain.ParseDate(exDate)
		if err != nil {
			panic(err)
		}
		d.Schedule.ExDividendDate = parsed
	}
	d.ID = domain.NewID(d.NaturalKey())
	return d
}

// Every failure the API returns must use the AIP-193 envelope: the error
// nested, the HTTP status as the code, and the canonical status name.
func TestErrorsUseTheAIP193Envelope(t *testing.T) {
	handler := newHandler(t, []domain.Company{{Symbol: "SNP", Name: "OMV PETROM S.A."}}, nil)

	tests := []struct {
		name       string
		path       string
		query      url.Values
		wantStatus int
		wantCode   string
	}{
		{
			name:       "unknown company",
			path:       "/v1/companies/NOPE",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "unknown path",
			path:       "/not-an-api-path",
			wantStatus: http.StatusNotFound,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "malformed resource name",
			path:       "/v1/companies/SNP/dividends/not-a-uuid",
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_ARGUMENT",
		},
		{
			name:       "filter that is not an expression",
			path:       "/v1/companies/-/dividends",
			query:      url.Values{"filter": {`year = 1) OR 1=1 --`}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_ARGUMENT",
		},
		{
			name:       "filter naming an undeclared field",
			path:       "/v1/companies/-/dividends",
			query:      url.Values{"filter": {`id = "x"`}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_ARGUMENT",
		},
		{
			name:       "ordering by an undeclared field",
			path:       "/v1/companies/-/dividends",
			query:      url.Values{"orderBy": {"secret_column"}},
			wantStatus: http.StatusBadRequest,
			wantCode:   "INVALID_ARGUMENT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := get(t, handler, tt.path, tt.query)
			if status != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %v)", status, tt.wantStatus, body)
			}

			errorObject, ok := body["error"].(map[string]any)
			if !ok {
				t.Fatalf("body has no nested error object: %v", body)
			}
			if got := errorObject["status"]; got != tt.wantCode {
				t.Errorf("status name = %v, want %q", got, tt.wantCode)
			}
			if got := errorObject["code"]; got != float64(tt.wantStatus) {
				t.Errorf("code = %v, want the HTTP status %d", got, tt.wantStatus)
			}
			if message, _ := errorObject["message"].(string); strings.TrimSpace(message) == "" {
				t.Error("error carries no message")
			}
		})
	}
}

// A dividend's resource name must identify exactly one dividend. Since the
// identifier alone locates it, a name pairing a real dividend with the wrong
// company has to be refused rather than resolved.
func TestGetDividendRejectsAMismatchedCompany(t *testing.T) {
	dividend := fixtureDividend("SNP", 2025, "2026-04-17")
	handler := newHandler(t,
		[]domain.Company{{Symbol: "SNP"}, {Symbol: "TLV"}},
		[]domain.Dividend{dividend})

	status, _ := get(t, handler, "/v1/companies/SNP/dividends/"+dividend.ID.String(), nil)
	if status != http.StatusOK {
		t.Fatalf("the correct name returned %d, want 200", status)
	}

	status, _ = get(t, handler, "/v1/companies/TLV/dividends/"+dividend.ID.String(), nil)
	if status != http.StatusNotFound {
		t.Errorf("a name with the wrong company returned %d, want 404", status)
	}
}

// Listing a named company that does not exist is a 404, not an empty page: the
// caller asked about a resource, and an empty list would read as an answer
// about a company that exists.
func TestListDividendsForAnUnknownCompanyIs404(t *testing.T) {
	handler := newHandler(t, []domain.Company{{Symbol: "SNP"}}, nil)

	if status, _ := get(t, handler, "/v1/companies/NOPE/dividends", nil); status != http.StatusNotFound {
		t.Errorf("status = %d, want 404", status)
	}
	// The wildcard names no company, so it is always a valid listing.
	if status, _ := get(t, handler, "/v1/companies/-/dividends", nil); status != http.StatusOK {
		t.Errorf("wildcard listing status = %d, want 200", status)
	}
}

// Amounts must reach the caller exactly, as google.type.Money rather than as a
// float. This is the field the Java DTO declared as Double.
func TestAmountsAreExactMoney(t *testing.T) {
	dividend := fixtureDividend("SNP", 2025, "2026-04-17")
	amount, err := domain.ParseAmount("0.1234")
	if err != nil {
		t.Fatalf("ParseAmount: %v", err)
	}
	dividend.Amounts.GrossPerShareNaturalPerson = amount

	handler := newHandler(t, []domain.Company{{Symbol: "SNP"}}, []domain.Dividend{dividend})
	status, body := get(t, handler, "/v1/companies/SNP/dividends/"+dividend.ID.String(), nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}

	money, ok := body["grossPerShareNaturalPerson"].(map[string]any)
	if !ok {
		t.Fatalf("amount is not an object: %v", body["grossPerShareNaturalPerson"])
	}
	if got := money["currencyCode"]; got != "RON" {
		t.Errorf("currency = %v, want RON", got)
	}
	if got := money["nanos"]; got != float64(123_400_000) {
		t.Errorf("nanos = %v, want 123400000", got)
	}
	// An amount BVB never reported must be absent, not zero.
	if _, present := body["totalAmount"]; present {
		t.Error("an unreported total was rendered rather than omitted")
	}
}

// A dividend with no ex-dividend date must be reachable. The Java API's
// date-range queries excluded these rows from every result.
func TestUndatedDividendsAreReachable(t *testing.T) {
	undated := fixtureDividend("SNP", 2026, "")
	scheduled := fixtureDividend("SNP", 2025, "2026-04-17")
	handler := newHandler(t, []domain.Company{{Symbol: "SNP"}}, []domain.Dividend{undated, scheduled})

	status, body := get(t, handler, "/v1/companies/-/dividends",
		url.Values{"filter": {"schedule.ex_dividend_date = null"}})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, body)
	}

	dividends, _ := body["dividends"].([]any)
	if len(dividends) != 1 {
		t.Fatalf("got %d dividends, want 1", len(dividends))
	}
	first, _ := dividends[0].(map[string]any)
	if got := first["state"]; got != "STATE_UNDATED" {
		t.Errorf("state = %v, want STATE_UNDATED", got)
	}
	if got := first["name"]; got != "companies/SNP/dividends/"+undated.ID.String() {
		t.Errorf("name = %v, want the undated dividend", got)
	}
}

// The page size is coerced down rather than rejected, as AIP-158 requires.
func TestPageSizeIsCoercedToTheMaximum(t *testing.T) {
	handler := newHandler(t, []domain.Company{{Symbol: "SNP"}}, nil)

	status, body := get(t, handler, "/v1/companies/-/dividends", url.Values{"pageSize": {"99999"}})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, body)
	}
}

func TestServesItsOwnSpecification(t *testing.T) {
	handler := newHandler(t, nil, nil)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.yaml", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	for _, fragment := range []string{"openapi: 3", "/v1/companies", "BVB Dividends API"} {
		if !strings.Contains(body, fragment) {
			t.Errorf("specification is missing %q", fragment)
		}
	}
}

func TestCORSAllowsOnlyConfiguredOrigins(t *testing.T) {
	handler := newHandler(t, nil, nil)

	tests := map[string]string{
		"http://localhost:4200": "http://localhost:4200",
		"https://evil.example":  "",
	}
	for origin, want := range tests {
		t.Run(origin, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/companies", nil)
			request.Header.Set("Origin", origin)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != want {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, want)
			}
		})
	}
}

func TestHealthAndReadiness(t *testing.T) {
	handler := newHandler(t, nil, nil)
	for _, path := range []string{"/healthz", "/readyz"} {
		if status, _ := get(t, handler, path, nil); status != http.StatusOK {
			t.Errorf("%s status = %d, want 200", path, status)
		}
	}
}
