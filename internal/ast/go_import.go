package ast

import (
	"crypto/sha256"
	"fmt"
)

// GoImportAlias is deterministic even before module linking. Named imports do
// not introduce a source namespace; their package identity remains explicit.
func GoImportAlias(importPath string) string {
	digest := sha256.Sum256([]byte(importPath))
	return fmt.Sprintf("__kinmokusei_go_%x", digest[:8])
}
