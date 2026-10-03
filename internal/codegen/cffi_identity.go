package codegen

import "strings"

// A resource's immutable self pointer distinguishes pointer aliases from
// copies of a wrapper. Reject copies before acquiring the ownership mutex or
// accessing the native pointer/context.
// Mutable ownership itself lives in separate shared state, so restoring a
// pre-close snapshot to the original address cannot resurrect a native owner.
func generateCFFIIdentityGuard(source *strings.Builder, name, closedFailure, copiedFailure string) {
	source.WriteString("if " + name + " == nil || " + name + ".self == nil { " + closedFailure + " }\n")
	source.WriteString("if " + name + ".self != " + name + " { " + copiedFailure + " }\n")
}
