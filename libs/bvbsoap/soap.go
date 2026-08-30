// Package bvbsoap talks to the Bucharest Stock Exchange financials web
// service and translates what it returns into the domain.
//
// It is the only package that knows BVB speaks SOAP. Everything it produces is
// domain types, so a change at BVB's end — or a move off SOAP entirely — is
// contained here.
//
// # On the wire format
//
// BVB publishes a WSDL at https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx?WSDL
// and the service this replaces generated its request and response classes from
// it at build time, so the element names were never written down in that
// repository either. The types in types.go are reconstructed from how the Java
// code used those generated classes, which pins every field, and are covered by
// the recorded responses in testdata/. Where BVB's real casing turns out to
// differ, types.go and mapper.go are the only files that need to change.
package bvbsoap

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Namespace is the XML namespace of the BVB financials service.
const Namespace = "http://www.bvb.ro/"

// soapEnvelope is the SOAP 1.1 envelope, used for both directions.
type soapEnvelope struct {
	XMLName xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Body    soapBody `xml:"http://schemas.xmlsoap.org/soap/envelope/ Body"`
}

type soapBody struct {
	Fault   *soapFault `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	Content []byte     `xml:",innerxml"`
}

// soapFault is a server-reported error.
type soapFault struct {
	Code   string `xml:"faultcode"`
	String string `xml:"faultstring"`
	Detail string `xml:"detail"`
}

// Error renders the fault as a Go error message.
func (f *soapFault) Error() string {
	message := strings.TrimSpace(f.String)
	if message == "" {
		message = "the service reported a fault with no description"
	}
	if code := strings.TrimSpace(f.Code); code != "" {
		return fmt.Sprintf("bvb fault %s: %s", code, message)
	}
	return "bvb fault: " + message
}

// maxResponseBytes caps how much of a response is read.
//
// The twenty-year backfill asks for every dividend a company ever declared, so
// the cap is generous, but it is still a cap: without one a misbehaving or
// misdirected endpoint could stream until the importer runs out of memory.
const maxResponseBytes = 32 << 20 // 32 MiB

// call performs one SOAP request and unmarshals the response body into out.
func (c *Client) call(ctx context.Context, action string, in, out any) error {
	body, err := marshalEnvelope(in)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", action, err)
	}
	request.Header.Set("Content-Type", "text/xml; charset=utf-8")
	request.Header.Set("SOAPAction", `"`+Namespace+action+`"`)
	request.Header.Set("Accept", "text/xml")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", action, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBytes))
		_ = response.Body.Close()
	}()

	payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("read %s response: %w", action, err)
	}

	// A SOAP fault arrives with status 500, so the body is parsed before the
	// status is judged: the fault carries the service's own explanation, and
	// it is what tells the retry loop this was a refusal rather than a
	// transport failure. Only if the body yields no fault does the status
	// stand on its own.
	err = unmarshalEnvelope(payload, out)
	var fault *soapFault
	if errors.As(err, &fault) {
		return fmt.Errorf("call %s: %w", action, err)
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("call %s: %s", action, response.Status)
	}
	if err != nil {
		return fmt.Errorf("decode %s response: %w", action, err)
	}
	return nil
}

// marshalEnvelope wraps a request body in a SOAP envelope.
func marshalEnvelope(in any) ([]byte, error) {
	inner, err := xml.Marshal(in)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"><soap:Body>`)
	buf.Write(inner)
	buf.WriteString(`</soap:Body></soap:Envelope>`)
	return buf.Bytes(), nil
}

// unmarshalEnvelope unwraps a SOAP envelope and decodes its body into out,
// turning a fault into an error.
func unmarshalEnvelope(payload []byte, out any) error {
	var envelope soapEnvelope
	if err := xml.Unmarshal(payload, &envelope); err != nil {
		return fmt.Errorf("parse soap envelope: %w", err)
	}
	if envelope.Body.Fault != nil {
		return envelope.Body.Fault
	}
	if err := xml.Unmarshal(envelope.Body.Content, out); err != nil {
		return fmt.Errorf("parse soap body: %w", err)
	}
	return nil
}
