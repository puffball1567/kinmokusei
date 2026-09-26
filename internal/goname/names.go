// Package goname shares source identifier lowering between linking and emission.
package goname

import "go/token"

// Identifier preserves ordinary source names and escapes Go keywords and
// callable predeclared identifiers. Link-time collision checks must use the
// same spelling as the emitter, not only the original source spelling.
func Identifier(name string) string {
	if token.Lookup(name).IsKeyword() || predeclaredCallable(name) {
		return name + "_"
	}
	return name
}

func predeclaredCallable(name string) bool {
	switch name {
	case "append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "panic", "print", "println", "real", "recover":
		return true
	default:
		return false
	}
}
