package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

// Static methods and accessors lower to package functions. Their bodies are
// part of Go's lexical initialization dependency graph, including closures.
func staticMemberDependency(owner, goName string) string {
	return "static " + owner + " " + goName
}

func staticFieldDependency(owner, name string) string {
	return "static field " + owner + "." + name
}

func classConstructionDependency(owner string) string {
	return "constructor " + owner
}

func (c *Checker) recordGlobalDependency(name string) {
	owner := c.globalDependencyOwner
	if owner == "" {
		return
	}
	if c.globalDependencies == nil {
		c.globalDependencies = map[string]map[string]bool{}
	}
	if c.globalDependencies[owner] == nil {
		c.globalDependencies[owner] = map[string]bool{}
	}
	c.globalDependencies[owner][name] = true
}

// Go initialization follows references through callable bodies. A cycle solely
// between functions is recursion; a cycle returning to stored global state is an
// invalid initialization, even when a reference occurs inside a closure.
func (c *Checker) checkGlobalInitializationCycles(program *ast.Program) {
	check := func(name, description string, span source.Span) {
		seen := map[string]bool{}
		pending := []string{name}
		cycle := false
		for len(pending) != 0 && !cycle {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[current] {
				continue
			}
			seen[current] = true
			for dependency := range c.globalDependencies[current] {
				if dependency == name {
					cycle = true
					break
				}
				pending = append(pending, dependency)
			}
		}
		if cycle {
			c.report(span, description+" has an initialization cycle")
		}
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.VariableDecl:
			if !declaration.FunctionBinding {
				check(declaration.Name, fmt.Sprintf("global %q", declaration.Name), declaration.NameSpan)
			}
		case *ast.ClassDecl:
			for _, field := range declaration.Fields {
				if field.Static {
					check(staticFieldDependency(declaration.Name, field.Name), fmt.Sprintf("static field %q", declaration.Name+"."+field.Name), field.NameSpan)
				}
			}
		}
	}
}
