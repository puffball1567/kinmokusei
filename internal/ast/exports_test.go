package ast

import (
	"testing"

	"github.com/puffball1567/kinmokusei/internal/source"
)

func TestSourceExportVisibility(t *testing.T) {
	t.Parallel()
	span := source.Span{Path: "library.km"}
	declarations := []Declaration{
		&FunctionDecl{Name: "name", NameSpan: span, Span: span}, &VariableDecl{Name: "name", NameSpan: span, Span: span},
		&ClassDecl{Name: "name", NameSpan: span, Span: span}, &StructDecl{Name: "name", NameSpan: span, Span: span},
		&InterfaceDecl{Name: "name", NameSpan: span, Span: span}, &TypeDecl{Name: "name", NameSpan: span, Span: span},
		&EnumDecl{Name: "name", NameSpan: span, Span: span},
	}
	for _, declaration := range declarations {
		if name, got := DeclarationBinding(declaration); name != "name" || got != span {
			t.Fatalf("%T: %s %+v", declaration, name, got)
		}
		for _, test := range []struct {
			exports []ExportDecl
			want    bool
		}{
			{nil, true}, {[]ExportDecl{{Span: span}}, false},
			{[]ExportDecl{{Span: source.Span{Path: "other.km"}}}, true},
			{[]ExportDecl{{Span: span, Names: []ExportName{{Name: "name"}}}}, true},
			{[]ExportDecl{{Span: span, Names: []ExportName{{Name: "source", ResolvedName: "name"}}}}, true},
			{[]ExportDecl{{Span: span, Names: []ExportName{{Name: "other"}}}}, false},
		} {
			if got := SourceExported(&Program{Exports: test.exports}, declaration); got != test.want {
				t.Fatalf("%T %v: %v want %v", declaration, test.exports, got, test.want)
			}
		}
	}
	for _, declaration := range []Declaration{nil, (*FunctionDecl)(nil), (*VariableDecl)(nil), (*ClassDecl)(nil), (*StructDecl)(nil), (*InterfaceDecl)(nil), (*TypeDecl)(nil), (*EnumDecl)(nil), &MethodDecl{}, &CABIExportDecl{}} {
		if name, span := DeclarationBinding(declaration); name != "" || span != (source.Span{}) || SourceExported(&Program{}, declaration) {
			t.Fatalf("unexpected binding %T", declaration)
		}
	}
}
