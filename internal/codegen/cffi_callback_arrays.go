package codegen

import (
	"strconv"
	"strings"
)

// Snapshot C elements into Go-owned storage before invoking user code. Never
// reinterpret a C POD layout as a Go struct or retain a native slice alias.
func cffiCallbackArrayConversion(parameter cffiParameter, index int, element cffiScalar, failureReturn string) (local, conversion, copyBack string) {
	suffix := strconv.Itoa(index)
	name := "value" + suffix
	length := name + "Length"
	local = "kinmokuseiCopiedArray" + suffix
	native := "kinmokuseiArraySource" + suffix
	var code strings.Builder
	code.WriteString("var " + local + " []" + element.goType + "\n")
	code.WriteString("if " + name + " == nil && " + length + " != 0 { state.recordInputError(" + strconv.Quote(parameter.Name) + ", \"null array pointer with non-zero length\"); " + failureReturn + " }\n")
	goElement := "kinmokuseiGoArrayElement" + suffix
	code.WriteString(cffiArrayCopyBounds(length, name, element.goType, goElement,
		"state.recordInputError("+strconv.Quote(parameter.Name)+", \"array byte size exceeds Go int\"); "+failureReturn))
	code.WriteString(local + " = make([]" + element.goType + ", int(" + length + "))\n")
	code.WriteString("if " + length + " != 0 {\n")
	code.WriteString(native + " := unsafe.Slice(" + name + ", int(" + length + "))\n")
	code.WriteString("for index, value := range " + native + " { " + local + "[index] = " + element.fromC("value") + " }\n}\n")
	if parameter.Type == "inoutArray" {
		copyBack = "if " + length + " != 0 {\n" + native + " := unsafe.Slice(" + name + ", int(" + length + "))\nfor index, value := range " + local + " { " + native + "[index] = " + element.toC("value") + " }\n}\n"
	}
	return local, code.String(), copyBack
}
