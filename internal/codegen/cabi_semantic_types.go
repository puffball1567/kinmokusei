package codegen

import (
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/cabi"
)

// The transport has already been checked by mapProgramCABIType. Use the
// checker's resolved integer values rather than re-evaluating source syntax.
func cabiSemanticType(program *ast.Program, ref ast.TypeRef, mapped cabiScalar) cabi.Type {
	for _, declaration := range program.Declarations {
		enumeration, ok := declaration.(*ast.EnumDecl)
		if !ok || enumeration.Name != ref.Name {
			continue
		}
		members := make([]cabi.EnumMember, len(enumeration.Members))
		for index, member := range enumeration.Members {
			members[index] = cabi.EnumMember{Name: member.Name, Value: member.ResolvedValue}
		}
		sort.Slice(members, func(i, j int) bool { return members[i].Name < members[j].Name })
		return cabi.Type{Kind: "enum", Members: members}
	}
	switch mapped.goType {
	case "bool":
		return cabi.Type{Kind: "boolean"}
	case "void":
		return cabi.Type{Kind: "void"}
	case "float32", "float64":
		return cabi.Type{Kind: "float"}
	default:
		return cabi.Type{Kind: "integer"}
	}
}
