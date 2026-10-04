package codegen

import (
	"go/types"
	"strconv"
	"strings"
)

// Include declarations that appear later in the manifest: parameters must not
// shadow types, import aliases, builtins, or generated helpers used in a body.
func cffiGoBindingNames(manifest cffiManifest) map[string]bool {
	names := make(map[string]bool)
	for name := range cffiReservedGoNames {
		names[name] = true
	}
	for _, name := range types.Universe.Names() {
		names[name] = true
	}
	for _, enum := range manifest.Enums {
		names[enum.Name] = true
		for _, value := range enum.Values {
			names[value.Name] = true
		}
	}
	for _, structure := range manifest.Structs {
		names[structure.Name] = true
	}
	for _, union := range manifest.TaggedUnions {
		names[union.Name] = true
	}
	for _, handle := range manifest.Handles {
		names[handle.Name] = true
	}
	for _, callback := range manifest.Callbacks {
		names[callback.Name] = true
	}
	for _, registration := range manifest.CallbackRegistrations {
		names[registration.Name] = true
		names["Register"+registration.Name] = true
	}
	for _, function := range manifest.Functions {
		names[function.Name] = true
	}
	return names
}

var cffiFunctionLocals = []string{"output", "outputLength", "status", "elementSize", "sourceValues", "result", "callbackError"}
var cffiRegistrationLocals = []string{"callback", "state", "context", "status", "result", "resultError"}

// Do not mutate the manifest, especially callback parameter names: input error
// diagnostics must keep the author's original names. Positional Go call types
// are unchanged by these deterministic, collision-free signature bindings.
func cffiGoParameters(parameters []cffiParameter, bindings map[string]bool, locals []string) []cffiParameter {
	reserved := make(map[string]bool, len(locals))
	for _, name := range locals {
		reserved[name] = true
	}
	occupied := make(map[string]bool, len(parameters))
	for _, parameter := range parameters {
		occupied[parameter.Name] = true
	}
	result := append([]cffiParameter(nil), parameters...)
	for index := range result {
		name := result[index].Name
		if result[index].manifestName == "" {
			result[index].manifestName = name
		}
		if name != "_" && !bindings[name] && !reserved[name] && !strings.HasPrefix(name, "kinmokusei") {
			continue
		}
		candidate := "kinmokuseiParameter" + strconv.Itoa(index)
		for occupied[candidate] || bindings[candidate] || reserved[candidate] {
			candidate += "_"
		}
		result[index].Name = candidate
		occupied[candidate] = true
	}
	return result
}
