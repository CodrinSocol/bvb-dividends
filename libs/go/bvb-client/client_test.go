package bvbclient_test

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

	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// quietLogger discards the retry notices, so the test output shows only
// failures.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type recordedRequest struct {
	SOAPAction  string
	ContentType string
	Body        string
}

// serveGolden starts a server that answers every request with the named
// recorded response, and records the requests it received.
func serveGolden(t *testing.T, name string, status int) (*bvbclient.Client, *[]recordedRequest) {
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

	client := bvbclient.New(
		bvbclient.WithEndpoint(server.URL),
		bvbclient.WithHTTPClient(server.Client()),
		bvbclient.WithLogger(quietLogger()),
		bvbclient.WithRetry(1, time.Millisecond),
	)
	return client, &received
}

// The client reports what BVB reported, unchanged: the recorded response names
// SNP twice and once in lower case, and carries one entry with no symbol at
// all. Deduplicating and normalising those is the importing slice's decision,
// not this library's.
func TestGetLastDividendsReturnsEveryIdentification(t *testing.T) {
	client, requests := serveGolden(t, "get_last_dividends.xml", http.StatusOK)

	identifications, err := client.GetLastDividends(t.Context(), 20)
	if err != nil {
		t.Fatalf("GetLastDividends: %v", err)
	}
	if len(identifications) != 4 {
		t.Fatalf("got %d identifications, want 4: %+v", len(identifications), identifications)
	}
	if got, want := identifications[0].Symbol, "SNP"; got != want {
		t.Errorf("first symbol = %q, want %q", got, want)
	}
	if got, want := identifications[0].CompanyName, "OMV PETROM S.A."; got != want {
		t.Errorf("first company name = %q, want %q", got, want)
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

func TestGetDividendsReadsEveryReportedField(t *testing.T) {
	client, requests := serveGolden(t, "get_dividends.xml", http.StatusOK)

	dividends, err := client.GetDividends(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("GetDividends: %v", err)
	}
	if len(dividends) != 3 {
		t.Fatalf("got %d dividends, want 3", len(dividends))
	}

	// A fully scheduled dividend. The values arrive as BVB wrote them, so the
	// total keeps every digit a float would have lost.
	full := dividends[0]
	if got, want := full.Year, 2024; got != want {
		t.Errorf("year = %d, want %d", got, want)
	}
	if got, want := deref(full.DividendForNaturalPersons), "0.0345"; got != want {
		t.Errorf("per-share amount = %q, want %q", got, want)
	}
	if got, want := deref(full.DividendsTotal), "2085123456.7890"; got != want {
		t.Errorf("total = %q, want %q exactly", got, want)
	}
	if got, want := deref(full.ExDividendDate), "2025-06-10T00:00:00"; got != want {
		t.Errorf("ex-dividend date = %q, want %q", got, want)
	}
	if got, want := full.MethodOfDividendDistribution, "Bank transfer / Depozitarul Central"; got != want {
		t.Errorf("distribution method = %q, want %q", got, want)
	}

	// A dividend announced but not yet scheduled. An absent element must stay a
	// nil pointer rather than becoming an empty string, so that the caller can
	// still tell "not reported" from "reported as nothing".
	announced := dividends[1]
	if announced.DividendsTotal != nil {
		t.Errorf("an unreported total was read as %q", *announced.DividendsTotal)
	}
	if announced.ExDividendDate != nil {
		t.Errorf("an unreported ex-dividend date was read as %q", *announced.ExDividendDate)
	}
	if announced.AnnouncementDate == nil {
		t.Error("a reported announcement date was read as absent")
	}

	// A value BVB reported in an unusable form is still handed over verbatim;
	// this library does not decide that it is unusable.
	partial := dividends[2]
	if partial.DividendForNaturalPersons == nil {
		t.Error("an unparseable amount was dropped instead of being passed through")
	}
	if got, want := deref(partial.DividendsTotal), "1500000"; got != want {
		t.Errorf("total = %q, want %q; a bad sibling field spoiled it", got, want)
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

	_, err := client.GetDividends(t.Context(), "SNP")
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

	client := bvbclient.New(
		bvbclient.WithEndpoint(server.URL),
		bvbclient.WithHTTPClient(server.Client()),
		bvbclient.WithLogger(quietLogger()),
		bvbclient.WithRetry(4, time.Millisecond),
	)

	if _, err := client.GetLastDividends(t.Context(), 1); err == nil {
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

	client := bvbclient.New(
		bvbclient.WithEndpoint(server.URL),
		bvbclient.WithHTTPClient(server.Client()),
		bvbclient.WithLogger(quietLogger()),
		bvbclient.WithRetry(4, time.Millisecond),
	)

	identifications, err := client.GetLastDividends(t.Context(), 1)
	if err != nil {
		t.Fatalf("GetLastDividends after retries: %v", err)
	}
	if calls != 3 {
		t.Errorf("made %d calls, want 3", calls)
	}
	if len(identifications) != 4 {
		t.Errorf("got %d identifications after retrying, want 4", len(identifications))
	}
}

func TestContextCancellationStopsRetrying(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := bvbclient.New(
		bvbclient.WithEndpoint(server.URL),
		bvbclient.WithHTTPClient(server.Client()),
		bvbclient.WithLogger(quietLogger()),
		bvbclient.WithRetry(10, time.Hour),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if _, err := client.GetLastDividends(ctx, 1); err == nil {
		t.Fatal("expected an error when the context expires")
	}
}

// A window of zero days is refused here rather than sent, because BVB answers
// it with an empty list, which reads downstream as "nothing was announced".
func TestNonPositiveWindowIsRefused(t *testing.T) {
	client, requests := serveGolden(t, "get_last_dividends.xml", http.StatusOK)

	if _, err := client.GetLastDividends(t.Context(), 0); err == nil {
		t.Fatal("a window of zero days was accepted")
	}
	if len(*requests) != 0 {
		t.Errorf("made %d requests, want none", len(*requests))
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
