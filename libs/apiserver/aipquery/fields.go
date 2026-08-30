// Package aipquery turns the standard AIP list parameters — the AIP-160
// filter, the AIP-132 order_by and the AIP-158 page token — into the domain's
// own query types.
//
// Filter expressions are parsed and type-checked by go.einride.tech/aip, which
// implements the AIP-160 grammar and produces a CEL expression tree. This
// package walks that tree and rebuilds it as a domain.Predicate over a closed
// set of fields. A field the caller names but this package does not declare is
// rejected during type-checking, long before anything reaches SQL.
package aipquery

import (
	"go.einride.tech/aip/filtering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// valueKind is the type of literal a field compares against.
type valueKind uint8

const (
	kindString valueKind = iota
	kindInt
	kindDate
	kindTimestamp
)

// field is one filterable and sortable attribute of a resource.
type field struct {
	// path is the name as it appears in a filter, matching the proto field
	// path, for example "schedule.ex_dividend_date".
	path string
	// target is the domain field it maps to.
	target domain.Field
	// kind is the literal type it accepts.
	kind valueKind
}

// dividendFields are the fields of a Dividend that may be filtered or sorted.
//
// Calendar dates are declared as strings so they can be written as
// "2026-04-17" in a filter, which is how AIP-160 filters over google.type.Date
// read; they are parsed into real dates here, not passed through as text.
var dividendFields = []field{
	{"company", domain.FieldCompany, kindString},
	{"year", domain.FieldYear, kindInt},
	{"dividend_type", domain.FieldDividendType, kindString},
	{"schedule.announcement_date", domain.FieldAnnouncementDate, kindDate},
	{"schedule.gms_reference_date", domain.FieldGMSReferenceDate, kindDate},
	{"schedule.gms_date", domain.FieldGMSDate, kindDate},
	{"schedule.record_date", domain.FieldRecordDate, kindDate},
	{"schedule.ex_dividend_date", domain.FieldExDividendDate, kindDate},
	{"schedule.payment_start_date", domain.FieldPaymentStartDate, kindDate},
	{"schedule.payment_end_date", domain.FieldPaymentEndDate, kindDate},
	{"create_time", domain.FieldCreateTime, kindTimestamp},
	{"update_time", domain.FieldUpdateTime, kindTimestamp},
}

// companyFields are the fields of a Company that may be filtered or sorted.
var companyFields = []field{
	{"symbol", domain.FieldSymbol, kindString},
	{"display_name", domain.FieldDisplayName, kindString},
	{"create_time", domain.FieldCreateTime, kindTimestamp},
	{"update_time", domain.FieldUpdateTime, kindTimestamp},
}

// nullIdent is the name a filter uses to ask whether BVB reported a field at
// all, as in `schedule.ex_dividend_date = null`.
//
// AIP-160's grammar has no null literal, so it arrives as a bare identifier
// and has to be declared for the expression to type-check. It is declared as a
// string because every nullable field this API exposes is a calendar date,
// which is itself declared as a string.
const nullIdent = "null"

// declarationType returns the CEL type a field's literals are checked against.
func (f field) declarationType() *expr.Type {
	switch f.kind {
	case kindInt:
		return filtering.TypeInt
	case kindTimestamp:
		return filtering.TypeTimestamp
	default:
		return filtering.TypeString
	}
}
