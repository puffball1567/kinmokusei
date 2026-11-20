package parser

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func TestAnonymousInterfaceTypeParse(t *testing.T) {
	program, count := parseSource(t, `function read(value: interface { read(offset: int): string; }): string { return value.read(0); }`)
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	parameter := program.Declarations[0].(*ast.FunctionDecl).Parameters[0].Type
	if !parameter.GoInterface || len(parameter.ObjectFields) != 1 || parameter.ObjectFields[0].Name != "read" {
		t.Fatalf("type=%#v", parameter)
	}
	if parameter.ObjectFields[0].Type.Return == nil || parameter.ObjectFields[0].Type.Return.Name != "string" {
		t.Fatalf("method=%#v", parameter.ObjectFields[0])
	}
}

func TestAnonymousInterfaceEmbeddingsParse(t *testing.T) {
	program, count := parseSource(t, `alias Reader = interface { io.Reader; interface { close(): void; }; size(): int; }`)
	if count != 0 {
		t.Fatalf("diagnostics=%d", count)
	}
	fields := program.Declarations[0].(*ast.TypeDecl).Underlying.ObjectFields
	if len(fields) != 3 || !fields[0].GoEmbedded || fields[0].Type.Qualifier != "io" || fields[0].Type.Name != "Reader" ||
		!fields[1].GoEmbedded || !fields[1].Type.GoInterface || fields[2].GoEmbedded || fields[2].Name != "size" {
		t.Fatalf("fields=%#v", fields)
	}
}
