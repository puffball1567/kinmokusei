package sema

import (
	"fmt"
	"sort"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

func (c *Checker) declareAbstractMethod(class *ast.ClassDecl, method *ast.MethodDecl) {
	if !method.Abstract {
		return
	}
	if !class.Abstract {
		c.report(method.Span, "abstract methods require an abstract class")
	}
	if method.Static || method.Final {
		c.report(method.Span, "abstract methods cannot be static or final")
	}
	if method.Visibility == ast.Private {
		c.report(method.Span, "abstract methods must be public or protected")
	}
	if !method.Override {
		method.Virtual = true
	}
}

func (c *Checker) checkConcreteClassMethods(class *ast.ClassDecl, symbol *classSymbol) {
	if class.Abstract {
		return
	}
	var missing []string
	for name, method := range symbol.methods {
		if method.abstract {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		c.report(class.NameSpan, fmt.Sprintf("concrete class %s must implement abstract method %s", class.Name, name))
	}
}
