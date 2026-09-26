package compiler

import "github.com/puffball1567/kinmokusei/internal/ast"

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
