package ast

// ReferenceReadSlot exposes checker-owned read metadata without changing the
// expression's source identity, lvalue shape or editor navigation information.
func ReferenceReadSlot(expression Expression) **TypeRef {
	switch value := expression.(type) {
	case *IdentifierExpr:
		return &value.NonNullRead
	case *UnaryExpr:
		return &value.NonNullRead
	case *BinaryExpr:
		return &value.NonNullRead
	case *GoTypeAssertionExpr:
		return &value.NonNullRead
	case *AwaitExpr:
		return &value.NonNullRead
	case *CallExpr:
		return &value.NonNullRead
	case *MemberExpr:
		return &value.NonNullRead
	case *IndexExpr:
		return &value.NonNullRead
	case *ClassUpcastExpr:
		return &value.NonNullRead
	}
	return nil
}
