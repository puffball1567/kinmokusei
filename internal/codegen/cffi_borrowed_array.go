package codegen

import (
	"strconv"
	"strings"
)

// Each input array is copied to C-owned memory for exactly one call. The
// generated pointer is never borrowed from the Go slice, and calloc clears
// struct padding before field-by-field conversion.
func generateCFFIBorrowedArrayPreparation(source *strings.Builder, function cffiFunction, handleResult bool, result cffiScalar, namedTypes map[string]cffiScalar) {
	for index, parameter := range function.Parameters {
		if parameter.Type != "borrowedArray" {
			continue
		}
		element := cffiTypeInfo(parameter.Element, namedTypes)
		name := "kinmokuseiArray" + strconv.Itoa(index)
		failureTooLarge := cffiFailureReturn(function, handleResult, result, "ErrBorrowedArrayTooLarge")
		failureAllocation := cffiFailureReturn(function, handleResult, result, "ErrBorrowedArrayAllocation")
		source.WriteString("var " + name + " *" + element.cgoType + "\n")
		source.WriteString("if len(" + parameter.Name + ") != 0 {\n")
		source.WriteString("elementSize := uint64(unsafe.Sizeof(*" + name + "))\n")
		source.WriteString("if uint64(len(" + parameter.Name + ")) > uint64(^uint(0)>>1)/elementSize || uint64(len(" + parameter.Name + ")) > uint64(^C.size_t(0))/elementSize { " + failureTooLarge + " }\n")
		source.WriteString("kinmokuseiBuffer := C.kinmokusei_cffi_alloc_array(C.size_t(len(" + parameter.Name + ")), C.size_t(elementSize))\n")
		source.WriteString("if kinmokuseiBuffer == nil { " + failureAllocation + " }\n")
		source.WriteString("defer C.kinmokusei_cffi_free_bytes(kinmokuseiBuffer)\n")
		source.WriteString(name + " = (*" + element.cgoType + ")(kinmokuseiBuffer)\n")
		source.WriteString("kinmokuseiValues := unsafe.Slice(" + name + ", len(" + parameter.Name + "))\n")
		source.WriteString("for index, value := range " + parameter.Name + " { kinmokuseiValues[index] = " + element.toC("value") + " }\n")
		source.WriteString("}\n")
	}
}
