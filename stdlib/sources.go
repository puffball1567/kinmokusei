// Package stdlib embeds the source-written Kinmokusei standard modules that
// ship in the compiler distribution.
package stdlib

import _ "embed"

const HTTPImportPath = "kinmokusei/http"
const InputImportPath = "kinmokusei/input"
const ConsoleImportPath = "kinmokusei/console"

// Source is one compiler-managed Kinmokusei source module.
type Source struct {
	ImportPath  string
	VirtualPath string
	Contents    string
}

//go:embed http/fetch.km
var httpSource string

//go:embed input/input.km
var inputSource string

//go:embed console/console.km
var consoleSource string

// Lookup returns an exact canonical standard-package source. Paths are not
// normalized so misspellings and traversal-like spellings cannot alias a
// compiler-managed package.
func Lookup(importPath string) (Source, bool) {
	switch importPath {
	case ConsoleImportPath:
		return Source{
			ImportPath:  ConsoleImportPath,
			VirtualPath: "@stdlib/kinmokusei/console/console.km",
			Contents:    consoleSource,
		}, true
	case InputImportPath:
		return Source{
			ImportPath:  InputImportPath,
			VirtualPath: "@stdlib/kinmokusei/input/input.km",
			Contents:    inputSource,
		}, true
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
