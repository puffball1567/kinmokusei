package ast

// LocalArrowGroup returns the consecutive direct arrow declarations at the
// beginning of statements. A non-arrow statement ends the group: constructing
// these closures cannot execute user code between their initializations.
func LocalArrowGroup(statements []Statement) []*VariableDecl {
	var group []*VariableDecl
	for _, statement := range statements {
		variable, ok := statement.(*VariableDecl)
		if !ok {
			break
		}
		if _, arrow := variable.Value.(*ArrowExpr); !arrow {
			break
		}
		group = append(group, variable)
	}
	return group
}
