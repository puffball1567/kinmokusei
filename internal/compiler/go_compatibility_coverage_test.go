package compiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// implementedGoCompatibilityContracts is the accepted runtime surface that has
// a direct Go equivalent. A feature is not complete until at least one isolated
// handwritten-Go differential scenario covers its contract.
var implementedGoCompatibilityContracts = []string{
	"core values, arithmetic, control flow, globals, arrows, objects, slices, maps, and interfaces",
	"relative modules and arrow calls",
	"classes, constructors, visibility, static methods, and collection access",
	"interface polymorphism across relative imports",
	"duplicate private dependency declarations and link-name isolation",
	"collection built-ins and evaluation behavior",
	"clear, min, max, named ordered values, NaN, signed zero, and evaluation order",
	"compiler built-in name shadowing",
	"fixed arrays, literals, map keys, copy, comparison, and imported arrays",
	"two/three-index slicing, aliasing, strings, named collections, evaluation order, and panic",
	"slice-to-array copy/view conversion, aliasing, zero length, and panic",
	"slice, array, map, string, channel, and pointer-to-array range",
	"bitwise operations, shifts, named types, signed behavior, and dynamic panic",
	"compound assignment, increment/decrement, target evaluation, named types, and division panic",
	"nullable values, narrowing, assignment, escape, capture, joins, and loop flow",
	"definite non-null constructor initialization across branches, switch, select, and guaranteed loops",
	"Result success, failure, forwarding, postfix propagation, explicit split binding, and interface dispatch",
	"ordinary value switches, grouping, identity, evaluation order, and break",
	"Go interface type switches, nil, bindings, and evaluation order",
	"defer order, goroutine execution, capture, and panic behavior",
	"directional channels, send/receive, close, checked receive, and channel range",
	"select send/receive/default, evaluation count, and nondeterministic outcomes",
	"standard-library functions, constants, variadics, formatting, paths, strings, and runtime calls",
	"Go callbacks, closures, package function values, and bound method values",
	"Go interface values and explicit class implementation",
	"generic Go function inference, explicit arguments, clone/copy behavior, and variadic expansion",
	"generic named Go types, methods, atomic pointers, and unique handles",
	"checked and unchecked Go interface assertions and panic behavior",
	"Go named types, pointers, structs, fields, methods, variables, assignments, and callbacks",
	"Go multiple results, blank bindings, reassignment, errors, and for initializers",
	"anonymous Go structs returned through fields and closures",
	"external Go modules through local replace, named values, methods, variadics, and narrow integers",
	"canonical Go import aliases across linked modules",
	"multiple-result assignment to a linked global",
	"canonical qualified Go types across linked modules",
	"locked project Go dependency execution",
	"locked target build-tag API selection",
	"locked cgo target loading and execution",
	"unsafe interop policy, nested signatures, fields, methods, and direct pointers",
	"unsafe built-ins, named pointers/slices, aliasing, layout, nil, identity, and panic",
	"HTTP/JSON application routing, encoding, status, Unicode, errors, and concurrent requests",
	"stable nullable class-member narrowing, assignments, nested paths, methods, aliases, joins, and loops",
	"unsigned fixed-width integers, conversions, arithmetic, bitwise operations, shifts, updates, ordering, overflow, and panic",
	"constructor definite initialization from boolean/integer/string constant expressions and append/makeSlice cardinality proofs",
	"constructor definite initialization through local, for-initializer, and global immutable constant bindings",
	"constructor definite initialization through explicitly imported immutable constant bindings",
	"nominal native structs, value copying, explicit pointer aliasing, shallow reference copying, comparability, recursive indirection, literal evaluation order, and empty-Go-interface use",
	"native struct value and pointer receiver methods, method values, nil receivers, selector behavior, evaluation order, and linked modules",
	"explicit-this external native struct receiver syntax, mixed declaration styles, value copying, pointer sharing, method values, chaining, and linked modules",
	"single class inheritance, base state and method reuse, explicit virtual override dispatch, super calls, constructor order, safe upcasts, nil, identity, multi-level classes, and inherited interfaces",
	"checked and forced class downcasts, nil and failure behavior, intermediate-base recovery, identity preservation, and single evaluation",
	"protected fields, constructor fields, instance/static methods, protected virtual dispatch, override, and derived-class access",
	"public cross-package class hierarchy construction, virtual calls, method values, nil and zero-value fallback, upcasts, checked/forced downcasts, and identity",
	"structured tasks with ordinary, void, and Result values, eager callee and argument evaluation, concurrent start, detach completion, await panic transport, and fatal detached panic",
	"Go-compatible byte-slice, rune-slice, named byte-slice, and integer-to-string conversions",
	"bounded context-aware HTTP fetch, status and header access, copied bodies, cancellation, close behavior, and concurrent tasks",
	"built-in and derived exceptions, ordered typed catches, bare rethrow, return-through-finally for ordinary and Result functions, nesting, nil errors, terminal flow, and runtime panic preservation",
	"public static class methods as idiomatic external Go functions with private static implementation isolation and Result signatures",
	"server-side HTTP routing with method patterns, path/query/header context, public Go handler use, conflict behavior, and concurrent requests",
	"native generic functions with inference, angle/bracket explicit and partial type arguments, collections, objects, callbacks, nullable values, value copying, pointer identity/dereference, evaluation order, panic behavior, Result propagation, linked modules, and external Go APIs",
	"nominal defined types and transparent aliases across strings, integers, slices, maps, fixed arrays, conversions, operators, map keys, mutation/aliasing, generics, Results, linked modules, and external Go APIs",
	"native generic structs with explicit instantiation, substituted fields and nested or external value/pointer receiver methods, method values, value copying, reference-bearing fields, nested and recursive pointers, class and defined-type arguments, map keys, Results, linked modules, and external Go APIs",
	"native generic interfaces with explicit instantiation, substituted method parameters and results, explicit class implementation, generic dispatch, reference identity, map keys, Results, multiple instantiations, linked modules, and external Go APIs",
	"native generic defined types with inferred comparable constraints, slices, maps, fixed arrays, nested and wrapped definitions, conversions, mutation and aliasing, map keys, generic functions, value/pointer receiver methods and method values, Results, multiple instantiations, linked modules, and external Go APIs",
	"non-generic defined type value and pointer receiver methods with mutation, automatic address-taking, method values, nil receivers, collection underlyings, Results, visibility, linked modules, Go method sets, and external Go APIs",
	"native variadic functions, generic functions, function types, arrows, constructors, interfaces, value receivers, defined types, individual arguments, spread calls, and virtual forwarding",
	"goto, forward and backward labels, labeled break and continue, labeled loops and switches, and control-transfer result behavior",
	"explicit comparable constraints on native generic functions, structs, interfaces, and defined types with inferred and explicit calls and constrained map keys",
	"checked map lookup declarations and reassignment across ordinary, generic, defined, and imported named Go maps, including nil, empty, present-zero, and missing cases with single evaluation",
	"value-switch fallthrough chains, default placement, unconditional next-clause entry, return flow, and generated Go behavior",
	"signed narrow int8/int16 constants, conversions, arithmetic, overflow, division, bitwise operations, shifts, updates, ordering, collections, defined types, generics, Go API shape, and panic behavior",
	"machine-width uint plus uint8/byte alias identity across constants, conversions, arithmetic, overflow, division, bitwise operations, shifts, updates, collections, defined types, generics, Go API shape, and panic behavior",
	"native integer enums with automatic and explicit values, signed and unsigned fixed-width boundaries, namespace members, conversions, switches, map keys, generics, ordering, value and pointer receiver methods, linked modules, and Go API shape",
	"native generic reference classes with explicit instantiation, substituted fields and methods, comparable constraints, method values, reference identity, interface implementation and dispatch, nested and reference-bearing values, linked modules, and external Go APIs",
	"recursive native defined types through slice, map, pointer, function, and channel indirection, including generic and mutually recursive declarations, receiver methods, named-function calls, linked modules, external Go APIs, and finite-size cycle rejection",
	"generic class static methods with inferred, explicit, partial, and constrained type arguments, private helpers, Result propagation, linked modules, and external Go APIs",
	"generic class inheritance with substituted base state, constructors, inherited methods and interfaces, concrete and remapped multi-level bases, descendant-aware typed upcasts and downcasts, identity, panic, and external Go APIs",
	"generic class virtual dispatch and explicit override across substituted, concrete, remapped, and multi-level bases, including construction phases, super calls, interfaces, method values, linked modules, and external Go APIs",
}

var differentialGoCompatibilityScenarios = map[string][]string{
	"behavior.test":                       {implementedGoCompatibilityContracts[0]},
	"generated.test":                      {implementedGoCompatibilityContracts[1]},
	"generated.class.test":                {implementedGoCompatibilityContracts[2]},
	"generated.interface.test":            {implementedGoCompatibilityContracts[3]},
	"linked.test":                         {implementedGoCompatibilityContracts[4]},
	"collections.test":                    {implementedGoCompatibilityContracts[5]},
	"ordered-clear.test":                  {implementedGoCompatibilityContracts[6]},
	"shadow.test":                         {implementedGoCompatibilityContracts[7]},
	"fixedarray.test":                     {implementedGoCompatibilityContracts[8]},
	"slicing.test":                        {implementedGoCompatibilityContracts[9]},
	"arrayconversion.test":                {implementedGoCompatibilityContracts[10]},
	"rangematrix.test":                    {implementedGoCompatibilityContracts[11]},
	"bitwise.test":                        {implementedGoCompatibilityContracts[12]},
	"updates.test":                        {implementedGoCompatibilityContracts[13]},
	"nullable.test":                       {implementedGoCompatibilityContracts[14]},
	"field-initialization.test":           {implementedGoCompatibilityContracts[15]},
	"result.test":                         {implementedGoCompatibilityContracts[16]},
	"valueswitch.test":                    {implementedGoCompatibilityContracts[17], implementedGoCompatibilityContracts[69]},
	"typeswitch.test":                     {implementedGoCompatibilityContracts[18]},
	"execution.test":                      {implementedGoCompatibilityContracts[19]},
	"channels.test":                       {implementedGoCompatibilityContracts[20]},
	"selection.test":                      {implementedGoCompatibilityContracts[21]},
	"interop.test":                        {implementedGoCompatibilityContracts[22]},
	"callbacks.test":                      {implementedGoCompatibilityContracts[23]},
	"interfaces.test":                     {implementedGoCompatibilityContracts[24]},
	"generics.test":                       {implementedGoCompatibilityContracts[25]},
	"generictypes.test":                   {implementedGoCompatibilityContracts[26]},
	"assertions.test":                     {implementedGoCompatibilityContracts[27]},
	"levelone.test":                       {implementedGoCompatibilityContracts[28]},
	"leveltwo.test":                       {implementedGoCompatibilityContracts[29]},
	"anonymousstruct.test":                {implementedGoCompatibilityContracts[30]},
	"externalmodule.test":                 {implementedGoCompatibilityContracts[31]},
	"aliases.test":                        {implementedGoCompatibilityContracts[32]},
	"multilink.test":                      {implementedGoCompatibilityContracts[33]},
	"qualifiedaliases.test":               {implementedGoCompatibilityContracts[34]},
	"example.com/application":             {implementedGoCompatibilityContracts[35]},
	"example.com/tagged-application":      {implementedGoCompatibilityContracts[36]},
	"example.com/cgo-application":         {implementedGoCompatibilityContracts[37]},
	"example.com/unsafe-application":      {implementedGoCompatibilityContracts[38]},
	"example.com/unsafe-builtins":         {implementedGoCompatibilityContracts[39]},
	"jsonapi.test":                        {implementedGoCompatibilityContracts[40]},
	"nullable-members.test":               {implementedGoCompatibilityContracts[41]},
	"unsigned.test":                       {implementedGoCompatibilityContracts[42], implementedGoCompatibilityContracts[71]},
	"constructor-constants.test":          {implementedGoCompatibilityContracts[43]},
	"constructor-bound-constants.test":    {implementedGoCompatibilityContracts[44]},
	"constructor-imported-constants.test": {implementedGoCompatibilityContracts[45]},
	"native-struct.test":                  {implementedGoCompatibilityContracts[46]},
	"native-struct-method.test":           {implementedGoCompatibilityContracts[47]},
	"external-native-receiver.test":       {implementedGoCompatibilityContracts[48]},
	"inheritance.test":                    {implementedGoCompatibilityContracts[49], implementedGoCompatibilityContracts[50], implementedGoCompatibilityContracts[51]},
	"example.com/hierarchy":               {implementedGoCompatibilityContracts[52]},
	"task.test":                           {implementedGoCompatibilityContracts[53]},
	"stringconversion.test":               {implementedGoCompatibilityContracts[54]},
	"fetch.test":                          {implementedGoCompatibilityContracts[55]},
	"exception.test":                      {implementedGoCompatibilityContracts[56]},
	"example.com/exceptions":              {implementedGoCompatibilityContracts[56]},
	"example.com/staticapi":               {implementedGoCompatibilityContracts[57]},
	"http-router.test":                    {implementedGoCompatibilityContracts[58]},
	"nativegenerics.test":                 {implementedGoCompatibilityContracts[59]},
	"nativegenerics-linked.test":          {implementedGoCompatibilityContracts[59]},
	"nativegenerics-public.test":          {implementedGoCompatibilityContracts[59]},
	"definedtypes.test":                   {implementedGoCompatibilityContracts[60]},
	"definedtypes-linked.test":            {implementedGoCompatibilityContracts[60]},
	"genericstruct.test":                  {implementedGoCompatibilityContracts[61]},
	"genericstruct-linked.test":           {implementedGoCompatibilityContracts[61]},
	"genericinterface.test":               {implementedGoCompatibilityContracts[62]},
	"genericinterface-linked.test":        {implementedGoCompatibilityContracts[62]},
	"genericdefined.test":                 {implementedGoCompatibilityContracts[63]},
	"genericdefined-linked.test":          {implementedGoCompatibilityContracts[63]},
	"definedtypemethod.test":              {implementedGoCompatibilityContracts[64]},
	"definedtypemethod-linked.test":       {implementedGoCompatibilityContracts[64]},
	"variadic.test":                       {implementedGoCompatibilityContracts[65]},
	"labels.test":                         {implementedGoCompatibilityContracts[66]},
	"genericconstraint.test":              {implementedGoCompatibilityContracts[67]},
	"checkedmap.test":                     {implementedGoCompatibilityContracts[68]},
	"signednarrow.test":                   {implementedGoCompatibilityContracts[70]},
	"enum.test":                           {implementedGoCompatibilityContracts[72]},
	"genericclass.test":                   {implementedGoCompatibilityContracts[73]},
	"genericclass-linked.test":            {implementedGoCompatibilityContracts[73], implementedGoCompatibilityContracts[75]},
	"recursivedefined.test":               {implementedGoCompatibilityContracts[74]},
	"recursivedefined-linked.test":        {implementedGoCompatibilityContracts[74]},
	"genericclassstatic.test":             {implementedGoCompatibilityContracts[75]},
	"genericinheritance.test":             {implementedGoCompatibilityContracts[76]},
	"genericinheritance-linked.test":      {implementedGoCompatibilityContracts[76]},
	"genericvirtual.test":                 {implementedGoCompatibilityContracts[77]},
	"genericvirtual-linked.test":          {implementedGoCompatibilityContracts[77]},
}

func TestImplementedGoCompatibilityCoverageIsComplete(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate compiler test directory")
	}
	directory := filepath.Dir(currentFile)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}

	helperNames := map[string]bool{
		"runGeneratedGoDifferentialTest":                 true,
		"runGeneratedGoDifferentialTestWithModule":       true,
		"runGeneratedGoDifferentialTestInExistingModule": true,
	}
	observed := map[string]string{}
	fileset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") || name == "differential_test.go" || name == filepath.Base(currentFile) {
			continue
		}
		path := filepath.Join(directory, name)
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(contents), "\"generated_test.go\"") {
			t.Errorf("%s writes a generated runtime test directly; use the isolated differential harness", name)
		}
		file, parseErr := parser.ParseFile(fileset, path, contents, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			if selector, selected := call.Fun.(*ast.SelectorExpr); selected && selector.Sel.Name == "Command" {
				if identifier, identified := selector.X.(*ast.Ident); identified && identifier.Name == "exec" && len(call.Args) >= 2 && stringLiteral(call.Args[0]) == "go" {
					operation := stringLiteral(call.Args[1])
					if operation == "test" || operation == "run" {
						t.Errorf("%s invokes go %s directly; use the isolated differential harness", name, operation)
					}
				}
			}
			identifier, identified := call.Fun.(*ast.Ident)
			if !identified || !helperNames[identifier.Name] || len(call.Args) < 3 {
				return true
			}
			scenario := stringLiteral(call.Args[2])
			if scenario == "" {
				t.Errorf("%s differential scenario must use a literal module identifier", name)
				return true
			}
			if previous, duplicate := observed[scenario]; duplicate {
				t.Errorf("differential scenario %q is duplicated in %s and %s", scenario, previous, name)
			} else {
				observed[scenario] = name
			}
			return true
		})
	}

	for scenario := range differentialGoCompatibilityScenarios {
		if _, exists := observed[scenario]; !exists {
			t.Errorf("registered Go compatibility scenario %q has no differential test", scenario)
		}
	}
	for scenario, file := range observed {
		if _, registered := differentialGoCompatibilityScenarios[scenario]; !registered {
			t.Errorf("differential scenario %q in %s is not classified in the implemented compatibility registry", scenario, file)
		}
	}

	coveredContracts := map[string][]string{}
	for scenario, contracts := range differentialGoCompatibilityScenarios {
		for _, contract := range contracts {
			coveredContracts[contract] = append(coveredContracts[contract], scenario)
		}
	}
	implemented := map[string]bool{}
	for _, contract := range implementedGoCompatibilityContracts {
		if implemented[contract] {
			t.Errorf("implemented Go compatibility contract is duplicated: %q", contract)
		}
		implemented[contract] = true
		if len(coveredContracts[contract]) == 0 {
			t.Errorf("implemented Go compatibility contract has no independent differential scenario: %q", contract)
		}
	}
	for contract, scenarios := range coveredContracts {
		if !implemented[contract] {
			t.Errorf("scenarios %v claim unknown Go compatibility contract %q", scenarios, contract)
		}
	}
}

func stringLiteral(expression ast.Expr) string {
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return value
}
