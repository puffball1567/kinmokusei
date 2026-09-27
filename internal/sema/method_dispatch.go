package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

// Methods and property accessors have the same inheritance contract. Keeping
// this check shared also preserves signature and construction-phase rules.
func (c *Checker) methodDispatchOwner(decl *ast.ClassDecl, method *ast.MethodDecl, signature Type, inherited methodSymbol, replaces bool) (string, bool) {
	name := method.Name
	if method.Accessor != "" {
		name = method.Accessor + " " + name
	}
	if method.Static && (method.Virtual || method.Override) {
		c.report(method.Span, "static methods cannot be virtual or override")
	}
	if len(method.TypeParameters) != 0 && (method.Virtual || method.Override || method.Final) {
		c.report(method.Span, "generic methods cannot be virtual, override, or final because Go method sets cannot represent method type parameters")
	}
	if method.Virtual && method.Override {
		c.report(method.Span, "override already remains virtual; remove the virtual modifier")
	}
	if method.Final && !method.Override {
		c.report(method.Span, "final methods must override an inherited virtual method")
	}
	if method.Virtual && method.Visibility == ast.Private {
		c.report(method.Span, "virtual methods must be public or protected")
	}
	owner := ""
	if replaces && inherited.declaringClass == decl.Name {
		c.report(method.Span, fmt.Sprintf("duplicate method %q", name))
		return "", false
	}
	if replaces {
		switch {
		case !method.Override:
			c.report(method.Span, fmt.Sprintf("method %q replaces inherited method from %s; add override", name, inherited.declaringClass))
		case inherited.final:
			c.report(method.Span, fmt.Sprintf("method %q in %s is final and cannot be overridden", name, inherited.declaringClass))
		case inherited.static:
			c.report(method.Span, fmt.Sprintf("static method %q cannot be overridden", name))
		case !inherited.virtual:
			c.report(method.Span, fmt.Sprintf("method %q in %s is not virtual", name, inherited.declaringClass))
		case method.Static:
			c.report(method.Span, fmt.Sprintf("override method %q cannot be static", name))
		case method.Visibility != inherited.visibility:
			c.report(method.Span, fmt.Sprintf("override method %q must preserve inherited visibility", name))
		case !identicalMethodSignature(signature, inherited.typeInfo):
			c.report(method.Span, fmt.Sprintf("override method %q has an incompatible signature", name))
		}
		owner = inherited.virtualOwner
	} else if method.Override {
		c.report(method.Span, fmt.Sprintf("method %q has override but no inherited method", name))
	}
	if method.Virtual && owner == "" {
		owner = decl.Name
	}
	return owner, true
}
