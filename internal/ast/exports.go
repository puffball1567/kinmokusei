package ast

import "github.com/puffball1567/kinmokusei/internal/source"

// ExportDecl controls source-module visibility, independently of Go and C ABI
// exports. An empty list still opts its source file into explicit exports.
type ExportDecl struct {
	Names  []ExportName
	Inline bool
	Span   source.Span
}

type ExportName struct {
	Name                string
	ResolvedName        string
	NameSpan            source.Span
	ResolvedDeclaration source.Span
}

// DeclarationBinding returns the standalone binding introduced by a declaration.
// Receiver methods and C ABI directives do not introduce module bindings.
func DeclarationBinding(declaration Declaration) (string, source.Span) {
	switch d := declaration.(type) {
	case *FunctionDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	case *VariableDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	case *ClassDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	case *StructDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	case *TypeDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	case *EnumDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	case *InterfaceDecl:
		if d != nil {
			return d.Name, d.NameSpan
		}
	}
	return "", source.Span{}
}

// SourceExported preserves legacy visibility only in files without source export
// directives. Paths are parser-owned source identities, also in merged programs.
func SourceExported(program *Program, declaration Declaration) bool {
	name, _ := DeclarationBinding(declaration)
	if name == "" {
		return false
	}
	explicit := false
	for _, exported := range program.Exports {
		if exported.Span.Path != declaration.GetSpan().Path {
			continue
		}
		explicit = true
		for _, selected := range exported.Names {
			resolved := selected.Name
			if selected.ResolvedName != "" {
				resolved = selected.ResolvedName
			}
			if resolved == name {
				return true
			}
		}
	}
	return !explicit
}
