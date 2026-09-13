package ast

import "github.com/puffball1567/kinmokusei/internal/source"

// ExportDecl controls source-module visibility, independently of Go and C ABI
// exports. An empty list still opts its source file into explicit exports.
type ExportDecl struct {
	Names        []ExportName
	Inline       bool
	Span         source.Span
	Path         string
	PathSpan     source.Span
	ResolvedPath string
}

type ExportName struct {
	Name                string
	Alias               string
	AliasSpan           source.Span
	ResolvedName        string
	NameSpan            source.Span
	ResolvedDeclaration source.Span
	// ReferencedDeclaration is the source spelling's rename identity. An
	// imported export alias has its own identity, separate from runtime storage.
	ReferencedDeclaration source.Span
}

func (n ExportName) PublicName() string {
	if n.Alias != "" {
		return n.Alias
	}
	return n.Name
}

func (n ExportName) PublicDeclaration() source.Span {
	if n.Alias != "" {
		return n.AliasSpan
	}
	if n.ReferencedDeclaration.Path != "" {
		return n.ReferencedDeclaration
	}
	return n.ResolvedDeclaration
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
		if exported.Path != "" {
			continue
		}
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
