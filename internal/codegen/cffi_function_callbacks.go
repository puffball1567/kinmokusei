package codegen

import (
	"strconv"
	"strings"
)

type cffiFunctionCallbackState struct {
	name, parameter string
}

func generateCFFIFunctionCallbackPreparation(source *strings.Builder, function cffiFunction, callbacks map[string]cffiCallback, handleResult bool, result cffiScalar) []cffiFunctionCallbackState {
	// Validate the entire callback list before creating any runtime handles.
	for _, parameter := range function.Parameters {
		if _, exists := callbacks[parameter.Type]; exists {
			failure := cffiFailureReturn(function, handleResult, result, "ErrNilCallback")
			source.WriteString("if " + parameter.Name + " == nil { " + failure + " }\n")
		}
	}
	var states []cffiFunctionCallbackState
	for index, parameter := range function.Parameters {
		callback, exists := callbacks[parameter.Type]
		if !exists {
			continue
		}
		state := "kinmokuseiCallbackState" + strconv.Itoa(index)
		manifestName := parameter.manifestName
		if manifestName == "" {
			manifestName = parameter.Name
		}
		states = append(states, cffiFunctionCallbackState{name: state, parameter: manifestName})
		source.WriteString(state + " := &kinmokuseiCFFI" + callback.Name + "State{callback: " + parameter.Name + "}\n")
		handle := "kinmokuseiCallbackHandle" + strconv.Itoa(index)
		source.WriteString(handle + " := cgo.NewHandle(" + state + ")\n")
		source.WriteString("defer " + handle + ".Delete()\n")
	}
	return states
}

func cffiFunctionCallbackFailure(function cffiFunction, states []cffiFunctionCallbackState, handleResult bool, result cffiScalar) string {
	if len(states) == 0 {
		return ""
	}
	var source strings.Builder
	// Keep the established single-callback error identity. With multiple
	// callback slots preserve every slot's first failure in declaration order.
	if len(states) == 1 {
		source.WriteString("if callbackError := " + states[0].name + ".callbackError(" + strconv.Quote(function.Name) + "); callbackError != nil { " + cffiFailureReturn(function, handleResult, result, "callbackError") + " }\n")
		return source.String()
	}
	source.WriteString("var kinmokuseiCallbackErrors []error\n")
	for _, state := range states {
		source.WriteString("if callbackError := " + state.name + ".callbackError(" + strconv.Quote(function.Name) + "); callbackError != nil { kinmokuseiCallbackErrors = append(kinmokuseiCallbackErrors, &CallbackArgumentError{Function: " + strconv.Quote(function.Name) + ", Parameter: " + strconv.Quote(state.parameter) + ", Err: callbackError}) }\n")
	}
	source.WriteString("if len(kinmokuseiCallbackErrors) != 0 { " + cffiFailureReturn(function, handleResult, result, "errors.Join(kinmokuseiCallbackErrors...)") + " }\n")
	return source.String()
}
