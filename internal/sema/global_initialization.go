package sema

import (
	"fmt"

	"github.com/puffball1567/kinmokusei/internal/ast"
)

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
	for _, declaration := range program.Declarations {
		variable, ok := declaration.(*ast.VariableDecl)
		if !ok || variable.FunctionBinding {
			continue
		}
		seen := map[string]bool{}
		pending := []string{variable.Name}
		cycle := false
		for len(pending) != 0 && !cycle {
			name := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if seen[name] {
				continue
			}
			seen[name] = true
			for dependency := range c.globalDependencies[name] {
				if dependency == variable.Name {
					cycle = true
					break
				}
				pending = append(pending, dependency)
			}
		}
		if cycle {
			c.report(variable.NameSpan, fmt.Sprintf("global %q has an initialization cycle", variable.Name))
		}
	}
}
