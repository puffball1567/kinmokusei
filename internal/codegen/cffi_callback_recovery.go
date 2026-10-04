package codegen

import "strings"

// With GODEBUG=panicnil=1, recover returns nil for panic(nil). Track whether
// user code returned independently of the recovered value. Input validation
// runs with the flag set, so legitimate early returns remain input errors.
func generateCFFICallbackRecovery(source *strings.Builder, callback cffiCallback, result cffiScalar) {
	source.WriteString("kinmokuseiCallbackReturned := true\n")
	source.WriteString("defer func() { value := recover(); if value != nil || !kinmokuseiCallbackReturned { state.recordPanic(value)\n")
	switch callback.Result {
	case "ownedBytes", "ownedArray":
		source.WriteString("if output != nil { C.free(unsafe.Pointer(output)) }; output = nil\nif outputLength != nil { *outputLength = 0 }\n")
	case "ownedCString":
		source.WriteString("output = nil\n")
	case "void":
		// The panic is retained in callback state; there is no C result to reset.
	default:
		source.WriteString("output = " + result.toC(result.zero) + "\n")
	}
	source.WriteString("} }()\n")
}
