package sema

import (
	"fmt"
	gotypes "go/types"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// An unrestricted operand contributes methods but no finite type-set terms.
// An empty restricted set is distinct from an unrestricted method interface.
type constraintOperand struct {
	terms      []*gotypes.Term
	shapes     []Type
	methods    []*gotypes.Func
	restricted bool
	valid      bool
}

func (c *Checker) resolveConstraintOperand(term ast.TypeSetTerm) constraintOperand {
	if operand, handled := c.sourceConstraintOperand(term); handled {
		return operand
	}
	resolved := c.resolveType(term.Type)
	if resolved.Kind == Invalid {
		return constraintOperand{}
	}
	// Source value interfaces have additional conformance, method-name and
	// nullable-signature rules. Do not erase those rules into a Go method set.
	if resolved.Kind != Interface && resolved.Kind != TypeParameter && resolved.Kind != Nullable {
		if goType, ok := goTypeOf(resolved); ok {
			if contract := underlyingGoInterface(goType); contract != nil {
				if term.Underlying {
					c.report(term.Span, "underlying constraint terms must name concrete types, not interfaces")
					return constraintOperand{}
				}
				if !contract.IsMethodSet() {
					c.report(term.Span, "imported constraint operands must be ordinary Go interfaces; use imported type-set constraints directly after 'extends'")
					return constraintOperand{}
				}
				if named, ok := gotypes.Unalias(goType).(*gotypes.Named); ok && len(term.Type.GenericArguments) != 0 {
					arguments := make([]Type, len(term.Type.GenericArguments))
					for i, ref := range term.Type.GenericArguments {
						arguments[i] = c.resolveType(ref)
					}
					if !c.checkConstraintMethodArguments(named.Origin(), arguments, term.Span) {
						return constraintOperand{}
					}
				}
				return constraintOperand{methods: constraintMethods(contract), valid: true}
			}
		}
	}
	terms, shapes, valid := c.resolveConstraintTerm(term, resolved)
	return constraintOperand{terms: terms, shapes: shapes, restricted: true, valid: valid}
}

func constraintMethods(contract *gotypes.Interface) []*gotypes.Func {
	methods := make([]*gotypes.Func, contract.NumMethods())
	for i := range methods {
		methods[i] = contract.Method(i)
	}
	return methods
}

func (c *Checker) mergeConstraintMethods(left, right []*gotypes.Func, span source.Span) ([]*gotypes.Func, bool) {
	valid := true
	for _, candidate := range right {
		found := false
		for _, existing := range left {
			// Id retains package identity for Go's unexported methods.
			if existing.Id() != candidate.Id() {
				continue
			}
			found = true
			if !gotypes.Identical(existing.Type(), candidate.Type()) {
				c.report(span, fmt.Sprintf("constraint intersection has incompatible signatures for method %s", candidate.Name()))
				valid = false
			}
			break
		}
		if !found {
			left = append(left, candidate)
		}
	}
	return left, valid
}
