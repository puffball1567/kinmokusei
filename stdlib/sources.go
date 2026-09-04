// Package stdlib embeds the source-written Kinmokusei standard modules that
// ship in the compiler distribution.
package stdlib

import _ "embed"

const HTTPImportPath = "kinmokusei/http"

// Source is one compiler-managed Kinmokusei source module.
type Source struct {
	ImportPath  string
	VirtualPath string
	Contents    string
}

//go:embed http/fetch.km
var httpSource string

// Lookup returns an exact canonical standard-package source. Paths are not
// normalized so misspellings and traversal-like spellings cannot alias a
// compiler-managed package.
func Lookup(importPath string) (Source, bool) {
	switch importPath {
	case HTTPImportPath:
		return Source{
			ImportPath:  HTTPImportPath,
			VirtualPath: "@stdlib/kinmokusei/http/fetch.km",
			Contents:    httpSource,
		}, true
	default:
		return Source{}, false
	}
}
