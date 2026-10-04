package codegen

// A native element can be smaller than the corresponding Go value: packed
// enums and POD layouts must be converted, not assumed layout-identical.
// Validate both byte products before any int conversion, slice, or allocation.
func cffiArrayCopyBounds(length, pointer, goType, goElement, failure string) string {
	return "var " + goElement + " " + goType + "\n" +
		"if uint64(" + length + ") > uint64(^uint(0)>>1)/uint64(unsafe.Sizeof(*" + pointer + ")) || uint64(" + length + ") > uint64(^uint(0)>>1)/uint64(unsafe.Sizeof(" + goElement + ")) { " + failure + " }\n"
}
