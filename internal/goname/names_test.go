package goname

import (
	"go/token"
	"testing"
)

func TestIdentifier(t *testing.T) {
	for _, name := range []string{"append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "panic", "print", "println", "real", "recover"} {
		if got := Identifier(name); got != name+"_" {
			t.Errorf("%s: %s", name, got)
		}
	}
	for keyword := token.BREAK; keyword <= token.VAR; keyword++ {
		name := keyword.String()
		if got := Identifier(name); got != name+"_" {
			t.Errorf("keyword %s: %s", name, got)
		}
	}
	for _, name := range []string{"value", "copy_", "func_", "値", "int", "string", "T", "_"} {
		if got := Identifier(name); got != name {
			t.Errorf("ordinary identifier %s: %s", name, got)
		}
	}
}

func FuzzIdentifierIsStable(f *testing.F) {
	for _, name := range []string{"copy", "copy_", "func", "func_", "value", "値"} {
		f.Add(name)
	}
	f.Fuzz(func(t *testing.T, name string) {
		lowered := Identifier(name)
		if Identifier(lowered) != lowered {
			t.Fatalf("unstable lowering: %q -> %q", name, lowered)
		}
		if token.IsIdentifier(name) && !token.IsIdentifier(lowered) {
			t.Fatalf("invalid lowered identifier %q", lowered)
		}
	})
}
