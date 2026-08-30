package bvbsoap_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/bvbsoap"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// quietLogger discards the warnings the mapper emits for unreadable values, so
// the test output shows only failures.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// serveGolden starts a server that answers every request with the named
// recorded response, and records the requests it received.
func serveGolden(t *testing.T, name string, status int) (*bvbsoap.Client, *[]recordedRequest) {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	received := make([]recordedRequest, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = append(received, recordedRequest{
			SOAPAction:  r.Header.Get("SOAPAction"),
			ContentType: r.Header.Get("Content-Type"),
			Body:        string(body),
		})
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(server.Close)

	client := bvbsoap.New(
		bvbsoap.WithEndpoint(server.URL),
		bvbsoap.WithHTTPClient(server.Client()),
		bvbsoap.WithLogger(quietLogger()),
		bvbsoap.WithRetry(1, time.Millisecond),
	)
	return client, &received
}

type recordedRequest struct {
	SOAPAction  string
	ContentType string
	Body        string
}

func TestRecentlyAnnouncing(t *testing.T) {
	client, requests := serveGolden(t, "get_last_dividends.xml", http.StatusOK)

	companies, err := client.RecentlyAnnouncing(context.Background(), 20)
	if err != nil {
		t.Fatalf("RecentlyAnnouncing: %v", err)
	}

	// SNP appears twice and once in lower case; the empty symbol is unusable.
	// The Java client called .distinct() on objects with no equals method, so
	// it deduplicated nothing and re-imported the company once per entry.
	if len(companies) != 2 {
		t.Fatalf("got %d companies, want 2: %+v", len(companies), companies)
	}
	if got, want := companies[0].Symbol, domain.Symbol("SNP"); got != want {
		t.Errorf("first symbol = %q, want %q", got, want)
	}
	if got, want := companies[1].Name, "BANCA TRANSILVANIA S.A."; got != want {
		t.Errorf("second name = %q, want %q (whitespace trimmed)", got, want)
	}

	if len(*requests) != 1 {
		t.Fatalf("made %d requests, want 1", len(*requests))
	}
	request := (*requests)[0]
	if got, want := request.SOAPAction, `"http://www.bvb.ro/GetLastDividends"`; got != want {
		t.Errorf("SOAPAction = %s, want %s", got, want)
	}
	if !strings.Contains(request.ContentType, "text/xml") {
		t.Errorf("Content-Type = %q, want text/xml", request.ContentType)
	}
	for _, fragment := range []string{"<GetLastDividends", `xmlns="http://www.bvb.ro/"`, "<noDays>20</noDays>"} {
		if !strings.Contains(request.Body, fragment) {
			t.Errorf("request body is missing %q:\n%s", fragment, request.Body)
		}
	}
}

func TestDividendsFor(t *testing.T) {
	client, requests := serveGolden(t, "get_dividends.xml", http.StatusOK)
	company := domain.Company{Symbol: "SNP", Name: "OMV PETROM S.A."}

	dividends, err := client.DividendsFor(context.Background(), company)
	if err != nil {
		t.Fatalf("DividendsFor: %v", err)
	}
	if len(dividends) != 3 {
		t.Fatalf("got %d dividends, want 3", len(dividends))
	}

	// A fully scheduled dividend.
	full := dividends[0]
	if got, want := full.Year, 2024; got != want {
		t.Errorf("year = %d, want %d", got, want)
	}
	if got, want := full.Amounts.GrossPerShareNaturalPerson.String(), "0.0345"; got != want {
		t.Errorf("per-share amount = %q, want %q", got, want)
	}
	// Exact to the last digit BVB reported, which a float would not be.
	wantTotal, err := domain.ParseAmount("2085123456.7890")
	if err != nil {
		t.Fatalf("ParseAmount: %v", err)
	}
	if !full.Amounts.Total.Equal(wantTotal) {
		t.Errorf("total = %q, want %q exactly", full.Amounts.Total, wantTotal)
	}
	if got, want := full.Schedule.ExDividendDate.String(), "2025-06-10"; got != want {
		t.Errorf("ex-dividend date = %q, want %q", got, want)
	}
	if got, want := full.Schedule.PaymentEndDate.String(), "2025-12-31"; got != want {
		t.Errorf("payment end date = %q, want %q", got, want)
	}
	if got, want := full.DistributionMethod, "Bank transfer / Depozitarul Central"; got != want {
		t.Errorf("distribution method = %q, want %q", got, want)
	}
	if full.ID != domain.NewID(full.NaturalKey()) {
		t.Error("identifier was not derived from the natural key")
	}

	// A dividend announced but not yet scheduled: absent, not zero.
	announced := dividends[1]
	if announced.Amounts.Total.Valid() {
		t.Error("an unreported total was read as a value")
	}
	if announced.Schedule.ExDividendDate.Valid() {
		t.Error("an unreported ex-dividend date was read as a value")
	}
	if got, want := announced.Schedule.AnnouncementDate.String(), "2026-02-18"; got != want {
		t.Errorf("announcement date = %q, want %q", got, want)
	}

	// One unreadable field must not cost the rest of the dividend its import.
	partial := dividends[2]
	if partial.Amounts.GrossPerShareNaturalPerson.Valid() {
		t.Error("an unparseable amount was read as a value")
	}
	if partial.Schedule.ExDividendDate.Valid() {
		t.Error("an unparseable date was read as a value")
	}
	if got, want := partial.Amounts.Total.String(), "1500000"; got != want {
		t.Errorf("total = %q, want %q; a bad sibling field spoiled it", got, want)
	}
	if got, want := partial.Schedule.RecordDate.String(), "2024-06-12"; got != want {
		t.Errorf("record date = %q, want %q; a bad sibling field spoiled it", got, want)
	}

	request := (*requests)[0]
	if got, want := request.SOAPAction, `"http://www.bvb.ro/GetDividends"`; got != want {
		t.Errorf("SOAPAction = %s, want %s", got, want)
	}
	for _, fragment := range []string{"<identityType>Symbol</identityType>", "<identity>SNP</identity>"} {
		if !strings.Contains(request.Body, fragment) {
			t.Errorf("request body is missing %q:\n%s", fragment, request.Body)
		}
	}
}

// A fault is the service refusing a request it understood, so it must surface
// as an error with BVB's own message rather than as an empty result. The Java
// clients logged the exception and returned an empty list, which read
// downstream as "this company has no dividends".
func TestSOAPFaultIsAnError(t *testing.T) {
	client, _ := serveGolden(t, "soap_fault.xml", http.StatusInternalServerError)

	_, err := client.DividendsFor(context.Background(), domain.Company{Symbol: "SNP"})
	if err == nil {
		t.Fatal("a SOAP fault was reported as success")
	}
	if !strings.Contains(err.Error(), "Invalid identity type") {
		t.Errorf("error = %v, want it to carry the fault message", err)
	}
}

// A fault means "understood and refused", so retrying it only wastes time and
// hammers the service.
func TestSOAPFaultIsNotRetried(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("testdata", "soap_fault.xml"))
	if err != nil {
		t.Fatalf("read fault: %v", err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client := bvbsoap.New(
		bvbsoap.WithEndpoint(server.URL),
		bvbsoap.WithHTTPClient(server.Client()),
		bvbsoap.WithLogger(quietLogger()),
		bvbsoap.WithRetry(4, time.Millisecond),
	)

	if _, err := client.RecentlyAnnouncing(context.Background(), 1); err == nil {
		t.Fatal("expected an error")
	}
	if calls != 1 {
		t.Errorf("made %d calls, want 1: a fault must not be retried", calls)
	}
}

// A dropped connection during an unattended nightly import should not cost a
// company its data, so transport failures are retried.
func TestTransportFailureIsRetried(t *testing.T) {
	payload, err := os.ReadFile(filepath.Join("testdata", "get_last_dividends.xml"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			// Close without a response, as a dropped connection would.
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("test server does not support hijacking")
				return
			}
			conn, _, hijackErr := hijacker.Hijack()
			if hijackErr != nil {
				t.Errorf("hijack: %v", hijackErr)
				return
			}
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	client := bvbsoap.New(
		bvbsoap.WithEndpoint(server.URL),
		bvbsoap.WithHTTPClient(server.Client()),
		bvbsoap.WithLogger(quietLogger()),
		bvbsoap.WithRetry(4, time.Millisecond),
	)

	companies, err := client.RecentlyAnnouncing(context.Background(), 1)
	if err != nil {
		t.Fatalf("RecentlyAnnouncing after retries: %v", err)
	}
	if calls != 3 {
		t.Errorf("made %d calls, want 3", calls)
	}
	if len(companies) != 2 {
		t.Errorf("got %d companies after retrying, want 2", len(companies))
	}
}

func TestContextCancellationStopsRetrying(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := bvbsoap.New(
		bvbsoap.WithEndpoint(server.URL),
		bvbsoap.WithHTTPClient(server.Client()),
		bvbsoap.WithLogger(quietLogger()),
		bvbsoap.WithRetry(10, time.Hour),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := client.RecentlyAnnouncing(ctx, 1); err == nil {
		t.Fatal("expected an error when the context expires")
	}
}
