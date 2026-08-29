package sema

import (
	"strings"
	"testing"
)

func TestCollectionBuiltinSemanticSuccessMatrix(t *testing.T) {
	tests := []struct {
		name, source string
	}{
		{"len matrix", `function use(text: string, slice: int[], array: [2]int, pointer: *[2]int, lookup: Map<string, int>, channel: GoChannel<int>): int { return len(text) + len(slice) + len(array) + len(pointer) + len(lookup) + len(channel); }`},
		{"cap matrix", `function use(slice: int[], array: [2]int, pointer: *[2]int, channel: GoChannel<int>): int { return cap(slice) + cap(array) + cap(pointer) + cap(channel); }`},
		{"named len cap", `import go net from "net"; import go http from "net/http"; function use(ip: net.IP, header: http.Header): int { return len(ip) + cap(ip) + len(header); }`},
		{"append individual and empty", `function use(values: int[]): int[] { const unchanged = append(values); return append(unchanged, 1, 2, 3); }`},
		{"append spread", `function use(values: int[], suffix: int[]): int[] { return append(values, suffix...); }`},
		{"append string to bytes", `function use(values: byte[]): byte[] { return append(values, "abc"...); }`},
		{"append named slice", `import go net from "net"; function use(values: net.IP, suffix: byte[]): net.IP { return append(values, suffix...); }`},
		{"copy slices and string", `function use(destination: int[], source: int[], bytes: byte[]): int { return copy(destination, source) + copy(bytes, "abc"); }`},
		{"copy named slices", `import go net from "net"; function use(destination: net.IP, source: byte[]): int { return copy(destination, source); }`},
		{"delete local and named maps", `import go http from "net/http"; function use(lookup: Map<string, int>, header: http.Header): void { delete(lookup, "x"); delete(header, "X-Test"); }`},
		{"clear slices maps nullable and named", `import go net from "net"; import go http from "net/http"; function use(values: int[], lookup: Map<string, int>, nullableValues: int[] | null, nullableMap: Map<string, int> | null, ip: net.IP, header: http.Header): void { clear(values); clear(lookup); clear(nullableValues); clear(nullableMap); clear(ip); clear(header); }`},
		{"min max ordered matrix", `import go time from "time"; function integers(left: int, right: int): int { return min(left, right, 0); } function text(left: string, right: string): string { return max(left, right, "m"); } function numbers(): float { return min(1, 2.5); } function duration(left: time.Duration, right: time.Duration): time.Duration { return max(left, right, 0); }`},
		{"make slice boundaries", `function use(length: int, capacity: int): int[][] { const empty = makeSlice[int](0); const sized = makeSlice[int](length, capacity); const nested = makeSlice[int[]](2); return append(nested, sized); }`},
		{"make map boundaries", `function use(capacity: int): Map<[2]byte, string> { const empty = makeMap[[2]byte, string](); const sized = makeMap[[2]byte, string](capacity); return sized; }`},
		{"builtin shadowing", `function len(value: int): int { return value + 1; } function use(): int { const copy = (value: int) => value + 2; return len(1) + copy(1); }`},
		{"all builtin names shadowed", `function len(value: int): int { return value; } function cap(value: int): int { return value; } function append(value: int): int { return value; } function copy(value: int): int { return value; } function delete(value: int): int { return value; } function clear(value: int): int { return value; } function min(value: int): int { return value; } function max(value: int): int { return value; } function makeSlice(value: int): int { return value; } function makeMap(value: int): int { return value; } function use(): int { return len(1) + cap(2) + append(3) + copy(4) + delete(5) + clear(6) + min(7) + max(8) + makeSlice(9) + makeMap(10); }`},
		{"slice to array copy and view", `function use(values: int[]): int { const copied: [2]int = copyArray[[2]int](values); const viewed: *[2]int = viewArray[[2]int](values); return copied[0] + viewed[1]; }`},
		{"slice to zero array", `function use(values: byte[]): int { const copied: [0]byte = copyArray[[0]byte](values); const viewed: *[0]byte = viewArray[[0]byte](values); return len(copied) + len(viewed); }`},
		{"named slice to array", `import go net from "net"; function use(values: net.IP): [4]byte { return copyArray[[4]byte](values); }`},
		{"named array target", `import go crc32 from "hash/crc32"; function copied(value: *crc32.Table): crc32.Table { return copyArray[crc32.Table](value[:]); } function viewed(value: *crc32.Table): *crc32.Table { return viewArray[crc32.Table](value[:]); }`},
		{"array conversion names shadowed", `function copyArray(value: int): int { return value + 1; } function viewArray(value: int): int { return value + 2; } function use(): int { return copyArray(1) + viewArray(1); }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestCollectionBuiltinSemanticFailureMatrix(t *testing.T) {
	tests := []struct {
		name, source, want string
	}{
		{"len missing argument", `function bad(): int { return len(); }`, "len expects 1 arguments, got 0"},
		{"len excess arguments", `function bad(values: int[]): int { return len(values, values); }`, "len expects 1 arguments, got 2"},
		{"len type arguments", `function bad(values: int[]): int { return len[int](values); }`, "len expects 0 type arguments, got 1"},
		{"len spread", `function bad(values: int[]): int { return len(values...); }`, "len does not accept spread arguments"},
		{"len target", `function bad(value: int): int { return len(value); }`, "len requires a string"},
		{"len untyped nil", `function bad(): int { return len(nil); }`, "len requires a string"},
		{"cap map", `function bad(value: Map<string, int>): int { return cap(value); }`, "cap requires an array"},
		{"cap string", `function bad(value: string): int { return cap(value); }`, "cap requires an array"},
		{"append missing destination", `function bad(): void { append(); }`, "append expects a destination slice"},
		{"append fixed array", `function bad(value: [2]int): void { append(value, 3); }`, "append requires a slice"},
		{"append untyped nil", `function bad(): void { append(nil, 3); }`, "append requires a slice"},
		{"append element", `function bad(value: int[]): void { append(value, "x"); }`, "cannot use string as int"},
		{"append spread source", `function bad(value: int[], item: int): void { append(value, item...); }`, "expanded append source must be a compatible slice"},
		{"append spread element", `function bad(value: int[], source: string[]): void { append(value, source...); }`, "does not match destination element"},
		{"append spread arity", `function bad(value: int[], left: int[], right: int[]): void { append(value, left, right...); }`, "spread append expects a destination and one expanded source"},
		{"append type arguments", `function bad(value: int[]): void { append[int](value, 1); }`, "append does not accept type arguments"},
		{"copy arity", `function bad(value: int[]): int { return copy(value); }`, "copy expects 2 arguments, got 1"},
		{"copy destination", `function bad(source: int[]): int { return copy(1, source); }`, "copy destination must be a slice"},
		{"copy source", `function bad(destination: int[]): int { return copy(destination, 1); }`, "copy source must be a compatible slice"},
		{"copy string to nonbyte", `function bad(destination: int[]): int { return copy(destination, "x"); }`, "copy source must be a compatible slice"},
		{"copy element", `function bad(destination: int[], source: string[]): int { return copy(destination, source); }`, "does not match destination element"},
		{"copy spread", `function bad(destination: int[], source: int[]): int { return copy(destination, source...); }`, "copy does not accept spread arguments"},
		{"delete arity", `function bad(value: Map<string, int>): void { delete(value); }`, "delete expects 2 arguments, got 1"},
		{"delete target", `function bad(): void { delete(1, 1); }`, "delete requires a map"},
		{"delete key", `function bad(value: Map<string, int>): void { delete(value, 1); }`, "cannot use integer literal as string"},
		{"clear missing argument", `function bad(): void { clear(); }`, "clear expects 1 arguments, got 0"},
		{"clear excess arguments", `function bad(values: int[]): void { clear(values, values); }`, "clear expects 1 arguments, got 2"},
		{"clear fixed array", `function bad(values: [2]int): void { clear(values); }`, "clear requires a slice or map"},
		{"clear string", `function bad(value: string): void { clear(value); }`, "clear requires a slice or map"},
		{"clear scalar", `function bad(value: int): void { clear(value); }`, "clear requires a slice or map"},
		{"clear type arguments", `function bad(values: int[]): void { clear[int](values); }`, "clear expects 0 type arguments, got 1"},
		{"clear spread", `function bad(values: int[]): void { clear(values...); }`, "clear does not accept spread arguments"},
		{"min empty", `function bad(): int { return min(); }`, "min expects at least 1 argument, got 0"},
		{"max bool", `function bad(value: boolean): boolean { return max(value, false); }`, "max requires ordered operands"},
		{"min slice", `function bad(value: int[]): int[] { return min(value, value); }`, "min requires ordered operands"},
		{"max mixed types", `function bad(value: int): int { return max(value, "x"); }`, "max operands must have one ordered type"},
		{"min distinct typed numbers", `function bad(left: int, right: int64): int { return min(left, right); }`, "min operands must have one ordered type"},
		{"max integer overflow", `function bad(value: byte): byte { return max(value, 256); }`, "integer constant 256 cannot be represented as byte"},
		{"min type arguments", `function bad(value: int): int { return min[int](value); }`, "min does not accept type arguments"},
		{"max spread", `function bad(values: int[]): int { return max(values...); }`, "max does not accept spread arguments"},
		{"makeSlice missing type", `function bad(): void { makeSlice(1); }`, "makeSlice expects one element type argument, got 0"},
		{"makeSlice excess type", `function bad(): void { makeSlice[int, string](1); }`, "makeSlice expects one element type argument, got 2"},
		{"makeSlice missing length", `function bad(): void { makeSlice[int](); }`, "makeSlice expects between 1 and 2 size arguments, got 0"},
		{"makeSlice excess size", `function bad(): void { makeSlice[int](1, 2, 3); }`, "makeSlice expects between 1 and 2 size arguments, got 3"},
		{"makeSlice size type", `function bad(): void { makeSlice[int]("x"); }`, "makeSlice size must be an integer"},
		{"makeSlice negative length", `function bad(): void { makeSlice[int](-1); }`, "makeSlice size cannot be negative"},
		{"makeSlice negative capacity", `function bad(): void { makeSlice[int](0, -1); }`, "makeSlice size cannot be negative"},
		{"makeSlice capacity order", `function bad(): void { makeSlice[int](2, 1); }`, "capacity cannot be smaller than length"},
		{"makeSlice size out of range", `function bad(): void { makeSlice[int](9223372036854775808); }`, "makeSlice size is out of range"},
		{"makeSlice void element", `function bad(): void { makeSlice[void](1); }`, "cannot be used as a slice element"},
		{"makeSlice spread", `function bad(values: int[]): void { makeSlice[int](values...); }`, "makeSlice does not accept spread arguments"},
		{"makeMap missing types", `function bad(): void { makeMap(1); }`, "makeMap expects key and value type arguments, got 0"},
		{"makeMap one type", `function bad(): void { makeMap[string](); }`, "makeMap expects key and value type arguments, got 1"},
		{"makeMap excess types", `function bad(): void { makeMap[string, int, byte](); }`, "makeMap expects key and value type arguments, got 3"},
		{"makeMap excess size", `function bad(): void { makeMap[string, int](1, 2); }`, "makeMap expects between 0 and 1 size arguments, got 2"},
		{"makeMap size type", `function bad(): void { makeMap[string, int](false); }`, "makeMap size must be an integer"},
		{"makeMap negative size", `function bad(): void { makeMap[string, int](-1); }`, "makeMap size cannot be negative"},
		{"makeMap key", `function bad(): void { makeMap[int[], string](); }`, "cannot be used as a Map key"},
		{"makeMap void key", `function bad(): void { makeMap[void, string](); }`, "cannot be used as a Map key"},
		{"makeMap void value", `function bad(): void { makeMap[string, void](); }`, "cannot be used as a Map value"},
		{"makeMap spread", `function bad(values: int[]): void { makeMap[string, int](values...); }`, "makeMap does not accept spread arguments"},
		{"copyArray missing target", `function bad(values: int[]): void { copyArray(values); }`, "copyArray expects one fixed array type argument, got 0"},
		{"copyArray excess target", `function bad(values: int[]): void { copyArray[[1]int, [2]int](values); }`, "copyArray expects one fixed array type argument, got 2"},
		{"copyArray nonarray target", `function bad(values: int[]): void { copyArray[int[]](values); }`, "copyArray target must be a fixed array type"},
		{"copyArray missing source", `function bad(): void { copyArray[[1]int](); }`, "copyArray expects one slice argument, got 0"},
		{"copyArray excess source", `function bad(values: int[]): void { copyArray[[1]int](values, values); }`, "copyArray expects one slice argument, got 2"},
		{"copyArray nonslice source", `function bad(value: [2]int): void { copyArray[[2]int](value); }`, "copyArray requires a slice source"},
		{"copyArray element mismatch", `function bad(values: string[]): void { copyArray[[2]int](values); }`, "cannot convert slice string[] to copyArray target [2]int"},
		{"copyArray spread", `function bad(values: int[]): void { copyArray[[2]int](values...); }`, "copyArray does not accept spread arguments"},
		{"viewArray nonarray target", `function bad(values: int[]): void { viewArray[int](values); }`, "viewArray target must be a fixed array type"},
		{"viewArray nonslice source", `function bad(value: int): void { viewArray[[1]int](value); }`, "viewArray requires a slice source"},
		{"viewArray element mismatch", `function bad(values: byte[]): void { viewArray[[2]int](values); }`, "cannot convert slice byte[] to viewArray target [2]int"},
		{"viewArray spread", `function bad(values: int[]): void { viewArray[[2]int](values...); }`, "viewArray does not accept spread arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}
