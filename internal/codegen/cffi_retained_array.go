package codegen

import "strings"

// The registration owns this C allocation, never the caller's Go slice.
// Rollback is installed by the caller before any input is allocated; native
// failure drains admitted callbacks before that rollback frees the arrays.
func cffiRetainedArrayPreparation(parameter cffiParameter, local string, element cffiScalar) string {
	var source strings.Builder
	source.WriteString("if len(" + parameter.Name + ") != 0 {\n")
	source.WriteString("kinmokuseiElementSize := uint64(unsafe.Sizeof(*" + local + "))\n")
	source.WriteString("if uint64(len(" + parameter.Name + ")) > uint64(^uint(0)>>1)/kinmokuseiElementSize || uint64(len(" + parameter.Name + ")) > uint64(^C.size_t(0))/kinmokuseiElementSize { return nil, ErrRetainedArrayTooLarge }\n")
	source.WriteString(local + " = (*" + element.cgoType + ")(C.kinmokusei_cffi_alloc_array(C.size_t(len(" + parameter.Name + ")), C.size_t(kinmokuseiElementSize)))\n")
	source.WriteString("if " + local + " == nil { return nil, ErrRetainedArrayAllocation }\n")
	source.WriteString("kinmokuseiValues := unsafe.Slice(" + local + ", len(" + parameter.Name + "))\n")
	source.WriteString("for index, value := range " + parameter.Name + " { kinmokuseiValues[index] = " + element.toC("value") + " }\n}\n")
	return source.String()
}
