package bvbclient

import "encoding/xml"

// The request and response types of the three BVB operations this client uses.
//
// Every name here is taken from the WSDL the service publishes at
// https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx?WSDL and is covered by the
// recorded responses in testdata/. Request parameters are PascalCase, which is
// worth stating because it is not what the previous service's generated
// classes suggested and it is not an error the service reports: a request
// whose parameters it cannot bind is answered with an empty result rather
// than a fault.

// ReportType selects which of an issuer's filings a balance operation asks
// about. The values are the service's own ReportTypeEnum.
type ReportType string

// The report types the service accepts.
const (
	ReportTypeAnnual     ReportType = "Annual"
	ReportTypeSemestrial ReportType = "Semestrial"
	ReportTypeQ1         ReportType = "Q1"
	ReportTypeQ3         ReportType = "Q3"
)

// getLastDividends asks for the companies that announced a dividend within the
// last NoDays days.
type getLastDividends struct {
	XMLName xml.Name `xml:"http://www.bvb.ro GetLastDividends"`
	NoDays  int      `xml:"NoDays"`
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
// BVB returns one of these per announced dividend and year, so a company that
// announced more than once appears more than once.
type Identification struct {
	// Symbol is the ticker as BVB reports it, which is not consistently
	// upper-case.
	Symbol string `xml:"Symbol"`

	// CompanyName is the full legal name, which BVB pads with whitespace.
	CompanyName string `xml:"Company>CompanyName"`

	// Year is the fiscal year the announcement relates to.
	Year int `xml:"Year"`
}

// identityTypeSymbol selects how the Identity field is interpreted. The
// service's SecurityIdentification enum accepts Symbol, ISIN or FiscalCode.
const identityTypeSymbol = "Symbol"

// getDividends asks for every dividend declared by one company.
type getDividends struct {
	XMLName      xml.Name `xml:"http://www.bvb.ro GetDividends"`
	IdentityType string   `xml:"IdentityType"`
	Identity     string   `xml:"Identity"`
}

// getDividendsResponse is the reply to getDividends.
type getDividendsResponse struct {
	XMLName xml.Name `xml:"GetDividendsResponse"`
	Result  struct {
		CompanyName string         `xml:"Company>CompanyName"`
		Infos       []DividendInfo `xml:"Dividends>DividendInfo"`
	} `xml:"GetDividendsResult"`
}

// DividendInfo is one dividend as BVB reports it.
//
// The numeric and date fields are carried as strings rather than as parsed
// types, so that one unparseable value spoils only its own field instead of
// the whole response. A value BVB has not reported arrives as an element
// carrying xsi:nil, which reads as an empty string: "not reported" and
// "reported as nothing" are the same thing on this wire, and neither is zero.
// Parsing them, and deciding what an empty one means, is the caller's job.
type DividendInfo struct {
	Year                      int    `xml:"Year"`
	DividendForNaturalPersons string `xml:"DividendForNaturalPersons"`
	DividendForLegalPersons   string `xml:"DividendForLegalPersons"`
	DividendsTotal            string `xml:"DividendsTotal"`
	DividendType              string `xml:"DividendType"`
	ReferenceDateForGMS       string `xml:"ReferenceDateForGMS"`

	// GMSDate is GMS_Date on the wire; it is the one element name the service
	// spells with an underscore.
	GMSDate string `xml:"GMS_Date"`

	RecordDate                   string `xml:"RecordDate"`
	ExDividendDate               string `xml:"ExDividendDate"`
	AnnouncementDate             string `xml:"AnnouncementDate"`
	StartPaymentDate             string `xml:"StartPaymentDate"`
	EndPaymentDate               string `xml:"EndPaymentDate"`
	MethodOfDividendDistribution string `xml:"MethodOfDividendDistribution"`
}

// getAvailableBalances asks which issuers filed a balance for a year.
type getAvailableBalances struct {
	XMLName    xml.Name   `xml:"http://www.bvb.ro GetAvailableBalances"`
	Year       int        `xml:"Year"`
	ReportType ReportType `xml:"ReportType"`
}

// getAvailableBalancesResponse is the reply to getAvailableBalances.
type getAvailableBalancesResponse struct {
	XMLName xml.Name `xml:"GetAvailableBalancesResponse"`
	Result  struct {
		Balances []SymbolBalance `xml:"SymbolBalanceTypeIdentification"`
	} `xml:"GetAvailableBalancesResult"`
}

// SymbolBalance is one issuer's filing for a year.
//
// It carries no company name: the service identifies the issuer by ticker here
// and only names it in the operations that return its data.
type SymbolBalance struct {
	Symbol      string     `xml:"Symbol"`
	Year        int        `xml:"Year"`
	ReportType  ReportType `xml:"ReportType"`
	BalanceType string     `xml:"BalanceType"`
}
