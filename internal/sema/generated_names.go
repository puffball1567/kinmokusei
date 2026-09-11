package sema

import (
	"fmt"
	gotoken "go/token"
	"unicode"
	"unicode/utf8"

	"github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/source"
)

func (c *Checker) checkGeneratedNames(program *ast.Program) {
	declared := map[string]source.Span{}
	structMembers := map[string]map[string]source.Span{}
	claim := func(name string, span source.Span) {
		if c.usesTasks && (name == "__kinmokuseiTask" || name == "__kinmokuseiVoidTask" || name == "__kinmokuseiResultTask" || name == "__kinmokuseiVoidResultTask") {
			c.report(span, fmt.Sprintf("generated Go name %q is reserved by the Task runtime", name))
			return
		}
		if c.usesExceptions && (name == "__kinmokuseiException" || name == "__kinmokuseiThrown" || name == "__kinmokuseiReturn" || name == "__kinmokuseiReturnValue" || name == "__kinmokuseiInitException" || name == "__kinmokuseiExceptionFromError" || name == "NewException") {
			c.report(span, fmt.Sprintf("generated Go name %q is reserved by the exception runtime", name))
			return
		}
		if previous, exists := declared[name]; exists {
			c.report(span, fmt.Sprintf("generated Go name %q collides with a declaration at %d:%d", name, previous.Start.Line, previous.Start.Column))
			return
		}
		declared[name] = span
	}
	claimStructMember := func(structName, name string, span source.Span) {
		if structName == "" || name == "" {
			return
		}
		members := structMembers[structName]
		if members == nil {
			members = map[string]source.Span{}
			structMembers[structName] = members
		}
		name = generatedIdentifier(name)
		if previous, exists := members[name]; exists {
			c.report(span, fmt.Sprintf("generated Go struct member name %q collides with a member at %d:%d", name, previous.Start.Line, previous.Start.Column))
			return
		}
		members[name] = span
	}
	for _, declaration := range program.Declarations {
		switch declaration := declaration.(type) {
		case *ast.FunctionDecl:
			claim(generatedIdentifier(declaration.Name), declaration.Span)
		case *ast.VariableDecl:
			claim(generatedIdentifier(declaration.Name), declaration.Span)
		case *ast.InterfaceDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("interface name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
		case *ast.ClassDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("class name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			c.checkOwnerTypeParameterNames(declaration.Name, declaration.TypeParameters)
			for _, method := range declaration.Methods {
				c.checkOwnerTypeParameterNames(declaration.Name, method.TypeParameters)
			}
			claim(declaration.Name, declaration.Span)
			claim("New"+declaration.Name, declaration.Span)
			claim("__kinmokuseiInit"+declaration.Name, declaration.Span)
			for _, field := range declaration.Fields {
				if field.Initializer != nil {
					helper := "__kinmokuseiFields" + declaration.Name
					claim(helper, declaration.Span)
					for _, parameter := range declaration.TypeParameters {
						if generatedIdentifier(parameter.Name) == helper {
							c.report(parameter.Span, "type parameter name conflicts with the generated class field initializer")
						}
					}
					if declaration.Constructor != nil {
						for _, parameter := range declaration.Constructor.Parameters {
							if generatedIdentifier(parameter.Name) == helper {
								c.report(parameter.Span, "constructor parameter name conflicts with the generated class field initializer")
							}
						}
					}
					break
				}
			}
			if declaration.Base != nil {
				claimStructMember(declaration.Name, declaration.Base.Name, declaration.Base.Span)
			}
			if declaration.HierarchyRoot == declaration.Name {
				claimStructMember(declaration.Name, "__kinmokuseiRoot", declaration.Span)
			}
			for _, owner := range declaration.VirtualOwners {
				if owner == declaration.Name {
					claimStructMember(declaration.Name, "__kinmokusei"+owner+"Self", declaration.Span)
				}
			}
			for _, field := range declaration.Fields {
				claimStructMember(declaration.Name, field.GoName, field.Span)
			}
			if declaration.Constructor != nil {
				for _, parameter := range declaration.Constructor.Parameters {
					if parameter.IsField {
						claimStructMember(declaration.Name, memberGoName(parameter.Name, parameter.Visibility), parameter.Span)
					}
				}
			}
			for _, ancestor := range declaration.Ancestors {
				claim("__kinmokuseiUpcast"+declaration.Name+"To"+ancestor, declaration.Span)
				claim("__kinmokuseiDowncast"+ancestor+"To"+declaration.Name, declaration.Span)
				claim("__kinmokuseiMustDowncast"+ancestor+"To"+declaration.Name, declaration.Span)
				claim("Upcast"+declaration.Name+"To"+ancestor, declaration.Span)
				claim("Downcast"+ancestor+"To"+declaration.Name, declaration.Span)
				claim("MustDowncast"+ancestor+"To"+declaration.Name, declaration.Span)
			}
			if len(declaration.Ancestors) != 0 {
				claim("__kinmokusei"+declaration.Name+"Projection", declaration.Span)
				claimStructMember(declaration.Name, "__kinmokuseiAs"+declaration.Name, declaration.Span)
			}
			for _, method := range declaration.Methods {
				if method.Virtual && !method.Override && !method.Static {
					claim("__kinmokusei"+declaration.Name+"Virtual", method.Span)
					break
				}
			}
			for _, method := range declaration.Methods {
				if method.Static {
					claim(staticMethodGoName(declaration.Name, method.GoName, method.Visibility), method.Span)
					continue
				}
				if len(method.TypeParameters) != 0 {
					claim(staticMethodGoName(declaration.Name, method.GoName, method.Visibility), method.Span)
					continue
				}
				if method.Virtual || method.Override {
					claimStructMember(declaration.Name, "__kinmokusei"+method.VirtualOwner+method.GoName, method.Span)
				}
				if !method.Override {
					claimStructMember(declaration.Name, method.GoName, method.Span)
				}
			}
		case *ast.StructDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("struct name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			for _, method := range declaration.Methods {
				if len(method.TypeParameters) != 0 {
					c.checkOwnerTypeParameterNames(declaration.Name, declaration.TypeParameters)
					c.checkOwnerTypeParameterNames(declaration.Name, method.TypeParameters)
				}
			}
			claim(declaration.Name, declaration.Span)
			for _, field := range declaration.Fields {
				claimStructMember(declaration.Name, field.GoName, field.Span)
			}
			for _, method := range declaration.Methods {
				if len(method.TypeParameters) != 0 {
					claim(staticMethodGoName(declaration.Name, method.GoName, method.Visibility), method.Span)
				} else {
					claimStructMember(declaration.Name, method.GoName, method.Span)
				}
			}
		case *ast.TypeDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("type name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
		case *ast.EnumDecl:
			if gotoken.Lookup(declaration.Name).IsKeyword() {
				c.report(declaration.Span, fmt.Sprintf("enum name %q is a Go keyword and cannot be generated", declaration.Name))
			}
			claim(declaration.Name, declaration.Span)
			for _, member := range declaration.Members {
				if member.Name == "_" {
					c.report(member.Span, "enum member name cannot be '_'")
					continue
				}
				claim(enumMemberGoName(declaration.Name, member.Name), member.Span)
			}
		case *ast.MethodDecl:
			receiver := declaration.ReceiverType
			if receiver.IsPointer() && receiver.Pointee != nil {
				receiver = *receiver.Pointee
			}
			claimStructMember(receiver.Name, declaration.GoName, declaration.Span)
		}
	}
	for _, export := range program.CABIExports {
		claim(export.Symbol, export.SymbolSpan)
	}
	seenImports := map[string]bool{}
	for _, imported := range program.Imports {
		if imported.Go && imported.Used {
			if seenImports[imported.Path] {
				continue
			}
			seenImports[imported.Path] = true
			alias := imported.Alias
			if imported.ResolvedAlias != "" {
				alias = imported.ResolvedAlias
			}
			claim(generatedIdentifier(alias), imported.Span)
		}
	}
}

func generatedIdentifier(name string) string {
	if gotoken.Lookup(name).IsKeyword() || isGeneratedGoPredeclaredName(name) {
		return name + "_"
	}
	return name
}

func isGeneratedGoPredeclaredName(name string) bool {
	switch name {
	case "append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max", "min", "new", "panic", "print", "println", "real", "recover":
		return true
	default:
		return false
	}
}

func memberGoName(name string, visibility ast.Visibility) string {
	if visibility != ast.Public {
		return name
	}
	r, size := utf8.DecodeRuneInString(name)
	if size == 0 {
		return name
	}
	return string(unicode.ToUpper(r)) + name[size:]
}

func staticMethodGoName(className, methodName string, visibility ast.Visibility) string {
	if visibility == ast.Public {
		return generatedIdentifier(className + methodName)
	}
	return generatedIdentifier("__kinmokuseiStatic" + className + methodName)
}
