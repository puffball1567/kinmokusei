// Package stdlib embeds the source-written OnsenTamago standard modules that
// ship in the compiler distribution.
package stdlib

import _ "embed"

const HTTPImportPath = "ontama/http"

// Source is one compiler-managed OnsenTamago source module.
type Source struct {
	ImportPath  string
	VirtualPath string
	Contents    string
}

//go:embed http/fetch.otm
var httpSource string

// Lookup returns an exact canonical standard-package source. Paths are not
// normalized so misspellings and traversal-like spellings cannot alias a
// compiler-managed package.
func Lookup(importPath string) (Source, bool) {
	switch importPath {
	case HTTPImportPath:
		return Source{
			ImportPath:  HTTPImportPath,
			VirtualPath: "@stdlib/ontama/http/fetch.otm",
			Contents:    httpSource,
		}, true
	default:
		return Source{}, false
	}
}
