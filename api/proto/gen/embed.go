// Package gen embeds the generated OpenAPI document.
//
// The document is generated from the protos, and embedding it here means a
// deployed binary always serves the contract it was built from. It lives in
// this package because go:embed cannot reach outside its own directory.
package gen

import _ "embed"

// OpenAPI is the generated OpenAPI 3.1 document describing the REST mapping of
// the service.
//
//go:embed bvb-dividends.openapi.yaml
var OpenAPI []byte
