package compiler

import (
	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/goname"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Internal scope keys cannot be source identifiers. Keeping an emitted-name
// index alongside source names makes capture checks constant-time, while
// source shadowing still uses only the original spelling.
func emittedBindingKey(name string) string { return "\x00go:" + goname.Identifier(name) }

type localBindings map[string]string

func cloneLocalBindings(names localBindings) localBindings {
	result := make(localBindings, len(names))
	for key, name := range names {
		result[key] = name
	}
	return result
}

func bindTypeParameter(visible moduleNames, name string) {
	visible[name] = typeParameterLinkBinding
	visible[emittedBindingKey(name)] = typeParameterLinkBinding
}

func (linker *sourceLinker) referenceName(name string, span source.Span, visible moduleNames, locals localBindings) string {
	if locals[name] == "" {
		linked := linker.name(name, span, visible)
		// Unbound builtin calls (len, copy, etc.) emit Go's original name,
		// not an escaped source declaration name. Named Go imports likewise
		// resolve separately to selectors rather than bare local identifiers.
		if visible[name] != "" && locals[emittedBindingKey(linked)] != "" {
			linker.capture(name, linked, span)
		}
		return linked
	}
	if winner := locals[emittedBindingKey(name)]; winner != "" && winner != name {
		linker.capture(name, winner, span)
	}
	return name
}

// Reuse the linker's lexical scope rules, including recursive arrow groups,
// loop/select/catch bindings and generic owners. Identity mappings leave source
// names and spans intact; export resolution runs only in the actual link pass.
func lexicalLinkNames(programs map[string]*ast.Program) map[string]bool {
	names := map[string]bool{}
	collector := &sourceLinker{lexicalNames: names}
	for _, program := range programs {
		identity := moduleNames{}
		for _, declaration := range program.Declarations {
			if name := topLevelName(declaration); name != "" {
				identity[name] = name
			}
		}
		for _, declaration := range program.Declarations {
			collector.linkDeclaration(declaration, identity, nil)
		}
	}
	return names
}
