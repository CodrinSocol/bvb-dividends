package bvbclient

import "encoding/xml"

// The request and response types of the two BVB operations this client uses.
//
// Every field here is determined by how the service this replaces consumed
// the classes its build generated from BVB's WSDL. Casing follows the .NET
// convention that ASMX services use — camelCase for request parameters,
// PascalCase for response members.

// getLastDividends asks for the companies that announced a dividend within the
// last noDays days.
type getLastDividends struct {
	XMLName xml.Name `xml:"http://www.bvb.ro/ GetLastDividends"`
	NoDays  int      `xml:"noDays"`
}

// getLastDividendsResponse is the reply to getLastDividends.
type getLastDividendsResponse struct {
	XMLName xml.Name `xml:"GetLastDividendsResponse"`
	Result  struct {
		Identifications []Identification `xml:"DividendIdentification"`
	} `xml:"GetLastDividendsResult"`
}

// Identification names one company that announced a dividend.
//
// BVB returns one of these per announced dividend, so a company that announced
// twice within the window appears twice.
type Identification struct {
	// Symbol is the ticker as BVB reports it, which is not consistently
	// upper-case.
	Symbol string `xml:"Symbol"`

	// CompanyName is the full legal name, which BVB pads with whitespace.
	CompanyName string `xml:"Company>CompanyName"`
}

// identityTypeSymbol selects how the identity field is interpreted.
//
// The WSDL is not reachable from the build environment to confirm the exact
// wire value, so this is the one value in this file that is a judgement
// rather than a derivation: .NET enum members are conventionally PascalCase,
// and BVB's own documentation writes it as "Symbol". If BVB rejects the
// request, this constant is the first thing to try changing.
const identityTypeSymbol = "Symbol"

// getDividends asks for every dividend declared by one company.
type getDividends struct {
	XMLName      xml.Name `xml:"http://www.bvb.ro/ GetDividends"`
	IdentityType string   `xml:"identityType"`
	Identity     string   `xml:"identity"`
}

// getDividendsResponse is the reply to getDividends.
type getDividendsResponse struct {
	XMLName xml.Name `xml:"GetDividendsResponse"`
	Result  struct {
		Infos []DividendInfo `xml:"Dividends>DividendInfo"`
	} `xml:"GetDividendsResult"`
}

// DividendInfo is one dividend as BVB reports it.
//
// The numeric and date fields are pointers to strings rather than parsed types
// so that "BVB did not report this" stays distinguishable from "BVB reported
// zero", and so that one unparseable value spoils only its own field instead of
// the whole response. Parsing them is the caller's job; this client does not
// decide what an unreadable value means.
type DividendInfo struct {
	Year                         int     `xml:"Year"`
	DividendForNaturalPersons    *string `xml:"DividendForNaturalPersons"`
	DividendForLegalPersons      *string `xml:"DividendForLegalPersons"`
	DividendsTotal               *string `xml:"DividendsTotal"`
	DividendType                 string  `xml:"DividendType"`
	ReferenceDateForGMS          *string `xml:"ReferenceDateForGMS"`
	GMSDate                      *string `xml:"GMSDate"`
	RecordDate                   *string `xml:"RecordDate"`
	ExDividendDate               *string `xml:"ExDividendDate"`
	AnnouncementDate             *string `xml:"AnnouncementDate"`
	StartPaymentDate             *string `xml:"StartPaymentDate"`
	EndPaymentDate               *string `xml:"EndPaymentDate"`
	MethodOfDividendDistribution string  `xml:"MethodOfDividendDistribution"`
}
