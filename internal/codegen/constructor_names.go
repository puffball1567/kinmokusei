package codegen

import "github.com/puffball1567/kinmokusei/internal/ast"

// Factory parameters must not hide the class type used to allocate the instance.
// Keep source names in the initializer, where the source constructor body runs.
func constructorFactoryParameterNames(class *ast.ClassDecl, parameters []ast.Parameter) []string {
	used := map[string]bool{"this": true, class.Name: true, initializerName(class.Name): true}
	for _, parameter := range class.TypeParameters {
		used[goName(parameter.Name)] = true
	}
	for _, parameter := range parameters {
		used[goName(parameter.Name)] = true
	}
	names := make([]string, len(parameters))
	for i, parameter := range parameters {
		name := goName(parameter.Name)
		if name == class.Name {
			for used[name] {
				name += "_"
			}
		}
		names[i] = name
		used[name] = true
	}
	return names
}
