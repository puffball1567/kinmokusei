package sema

import (
	"strings"
	"testing"
)

func TestChecksGoStandardLibraryInterop(t *testing.T) {
	diagnostics := checkSource(t, `
import go strings from "strings";
import go strconv from "strconv";
import go math from "math";
import go runtime from "runtime";
const ratio: float = math.Pi;
function normalize(value: string): string { return strings.ToUpper(strings.TrimSpace(value)); }
function words(value: string): string[] { return strings.Fields(value); }
function digits(value: int): string { return strconv.Itoa(value); }
function collect(): void { runtime.GC(); }
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestChecksGoNamedTypesPointersFieldsMethodsAndVariables(t *testing.T) {
	diagnostics := checkSource(t, `
import go bytes from "bytes";
import go http from "net/http";
import go os from "os";
import go strings from "strings";
import go time from "time";
function pointerRoundTrip(value: time.Duration): time.Duration {
  let copy: time.Duration = value;
  const pointer: *time.Duration = &copy;
  return *pointer;
}
function timeout(): time.Duration {
  const client: http.Client = http.Client{ Timeout: time.Second * 2 };
  return client.Timeout;
}
function nilClient(): boolean {
  let client: *http.Client = nil;
  return client == nil;
}
function currentUnix(): int64 { return time.Now().Unix(); }
function outputName(): string { return os.Stdout.Name(); }
function replaceOutput(output: *os.File): void { os.Stdout = output; }
function preserveAlias(value: os.FileInfo): os.FileInfo { return value; }
function readerLength(): int { return strings.NewReader("abc").Len(); }
function resetBuffer(): string {
  let buffer: bytes.Buffer = bytes.Buffer{};
  buffer.Reset();
  return buffer.String();
}
function duration(value: int64): time.Duration { return time.Duration(value); }
function inferredCallback(): time.Duration {
  const readTimeout = () => http.DefaultClient.Timeout;
  return readTimeout();
}
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestChecksGoMultipleResultsBlankBindingsReassignmentAndError(t *testing.T) {
	diagnostics := checkSource(t, `
import go errors from "errors";
import go strconv from "strconv";
function parse(value: string): int {
  const [parsed, err] = strconv.Atoi(value);
  if (err != nil) { return -1; }
  return parsed;
}
function reparse(value: string): int {
  let parsed: int = 0;
  let err: error = nil;
  [parsed, err] = strconv.Atoi(value);
  if (err != nil) { return -1; }
  return parsed;
}
function message(value: string): string {
  const [_, err] = strconv.Atoi(value);
  if (err == nil) { return ""; }
  return err.Error();
}
function discard(value: string): void { const [_, _] = strconv.Atoi(value); }
function makeError(): error { return errors.New("boom"); }
`)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %v", diagnostics)
	}
}

func TestGoInteropLevelOneFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"unknown qualified type", `import go time from "time"; function value(input: time.Missing): void {}`, "has no exported type"},
		{"non-type symbol in type position", `import go time from "time"; function value(input: time.Second): void {}`, "has no exported type"},
		{"nil to non-nilable", `function value(): int { let item: int = nil; return item; }`, "cannot use nil as int"},
		{"untyped nil variable", `function value(): int { const item = nil; return 1; }`, "cannot infer a variable type from nil"},
		{"nil compared with nil", `function value(): boolean { return nil == nil; }`, "cannot compare nil with nil"},
		{"nil array inference", `function value(): int { const items = [nil]; return 1; }`, "cannot infer an array element type from nil"},
		{"nil object inference", `function value(): int { const item = { pointer: nil }; return 1; }`, "cannot infer object field"},
		{"nil arrow inference", `function value(): int { const make = () => nil; return 1; }`, "cannot infer an arrow function return type from nil"},
		{"cross-package inferred type without import", `import go http from "net/http"; function value(): int { const readTimeout = () => http.DefaultClient.Timeout; return 1; }`, "requires an explicit import go alias"},
		{"address of call", `import go time from "time"; function value(): int { const pointer = &time.Now(); return 1; }`, "addressable operand"},
		{"address of Go constant", `import go time from "time"; function value(): int { const pointer = &time.Second; return 1; }`, "addressable operand"},
		{"dereference non-pointer", `function value(): int { let item: int = 1; return *item; }`, "requires a pointer operand"},
		{"different named types", `import go time from "time"; function value(): time.Duration { let result: time.Duration = time.January; return result; }`, "cannot use time.Month as time.Duration"},
		{"unknown struct field", `import go http from "net/http"; function value(): int { const client = http.Client{ Missing: 1 }; return 1; }`, "has no field"},
		{"unexported struct field", `import go time from "time"; function value(): int { const location = time.Location{ name: "x" }; return 1; }`, "is not exported"},
		{"duplicate struct field", `import go http from "net/http"; function value(): int { const client = http.Client{ Timeout: 1, Timeout: 2 }; return 1; }`, "duplicate Go struct field"},
		{"non-struct composite", `import go time from "time"; function value(): int { const duration = time.Duration{}; return 1; }`, "is not a struct"},
		{"wrong struct field type", `import go http from "net/http"; function value(): int { const client = http.Client{ Timeout: "slow" }; return 1; }`, "cannot use string as time.Duration"},
		{"missing method", `import go time from "time"; function value(): int { const now = time.Now(); return now.Missing(); }`, "has no exported member"},
		{"pointer method on temporary", `import go bytes from "bytes"; function value(): void { bytes.Buffer{}.Reset(); }`, "requires an addressable"},
		{"type used as value", `import go strings from "strings"; function value(): int { const reader = strings.Reader; return 1; }`, "cannot be used as a value"},
		{"assign Go constant", `import go time from "time"; function value(): void { time.Second = time.Minute; }`, "cannot assign to Go constant"},
		{"assign Go method", `import go bytes from "bytes"; function value(): void { let buffer = bytes.Buffer{}; buffer.Reset = () => {}; }`, "is not assignable"},
		{"assign field of temporary", `import go http from "net/http"; function value(): void { http.Client{}.Timeout = 1; }`, "is not assignable"},
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

func TestGoInteropSemanticFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"missing external package", `import go external from "example.com/external"; function value(): int { return 1; }`, "cannot load Go package"},
		{"unsafe", `import go unsafe from "unsafe"; function value(): int { return 1; }`, "requires [go.interop]"},
		{"unknown member", `import go strings from "strings"; function value(): string { return strings.Unknown("x"); }`, "has no exported member"},
		{"multiple return", `import go os from "os"; function value(): string { return os.Getwd(); }`, "require destructuring"},
		{"variadic too few fixed arguments", `import go fmt from "fmt"; function value(): void { fmt.Fprintf(); }`, "expects at least 2 arguments, got 0"},
		{"type member", `import go strings from "strings"; function value(): int { const reader = strings.Reader; return 1; }`, "cannot be used as a value"},
		{"namespace value", `import go strings from "strings"; function value(): int { const packageValue = strings; return 1; }`, "cannot be used as a value"},
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

func TestUnsafeGoInteropExplicitPolicy(t *testing.T) {
	source := `import go unsafe from "unsafe"; function identity(value: unsafe.Pointer): unsafe.Pointer { return value; }`
	if diagnostics := checkSource(t, source); !strings.Contains(strings.Join(diagnostics, "\n"), "requires [go.interop]") {
		t.Fatalf("deny diagnostics=%v", diagnostics)
	}
	if diagnostics := checkSourceWithPolicy(t, source, GoInteropPolicy{AllowUnsafe: true}); len(diagnostics) != 0 {
		t.Fatalf("allow diagnostics=%v", diagnostics)
	}
}

func TestUnsafeGoBuiltinSuccessMatrix(t *testing.T) {
	source := `
import go reflect from "reflect";
import go danger from "unsafe";
function size(value: int64): int { return int(danger.Sizeof(value)); }
function alignment(value: int64): int { return int(danger.Alignof(value)); }
function offset(value: reflect.StringHeader): int { return int(danger.Offsetof(value.Data)); }
function add(pointer: danger.Pointer, offset: int): danger.Pointer { return danger.Add(pointer, offset); }
function bytes(pointer: *byte, length: int): byte[] { return danger.Slice(pointer, length); }
function first(values: byte[]): *byte { return danger.SliceData(values); }
function text(pointer: *byte, length: int): string { return danger.String(pointer, length); }
function textData(value: string): *byte { return danger.StringData(value); }
`
	if diagnostics := checkSourceWithPolicy(t, source, GoInteropPolicy{AllowUnsafe: true}); len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}

func TestUnsafeGoBuiltinFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"sizeof missing argument", `function value(): int { return int(danger.Sizeof()); }`, "danger.Sizeof expects 1 arguments, got 0"},
		{"sizeof extra argument", `function value(): int { return int(danger.Sizeof(1, 2)); }`, "danger.Sizeof expects 1 arguments, got 2"},
		{"sizeof nil", `function value(): int { return int(danger.Sizeof(nil)); }`, "requires a typed value"},
		{"sizeof void", `import go runtime from "runtime"; function value(): int { return int(danger.Sizeof(runtime.GC())); }`, "requires a value, got void"},
		{"sizeof explicit type argument", `function value(): int { return int(danger.Sizeof[int](1)); }`, "expects 0 type arguments, got 1"},
		{"sizeof spread", `function value(values: int[]): int { return int(danger.Sizeof(values...)); }`, "does not accept spread arguments"},
		{"alignof missing argument", `function value(): int { return int(danger.Alignof()); }`, "danger.Alignof expects 1 arguments, got 0"},
		{"offsetof non-selector", `function value(): int { return int(danger.Offsetof(1)); }`, "requires a Go struct field selector"},
		{"offsetof method", `import go clock from "time"; function value(input: clock.Time): int { return int(danger.Offsetof(input.Unix)); }`, "requires a Go struct field selector"},
		{"add wrong pointer", `function value(pointer: *byte): danger.Pointer { return danger.Add(pointer, 1); }`, "pointer must be unsafe.Pointer"},
		{"add wrong offset", `function value(pointer: danger.Pointer): danger.Pointer { return danger.Add(pointer, "one"); }`, "offset must be an integer"},
		{"add offset out of range", `function value(pointer: danger.Pointer): danger.Pointer { return danger.Add(pointer, 999999999999999999999999999999); }`, "offset is out of range"},
		{"slice unsafe pointer", `function value(pointer: danger.Pointer): byte[] { return danger.Slice(pointer, 1); }`, "must be a typed Go pointer"},
		{"slice non-pointer", `function value(): byte[] { return danger.Slice(1, 1); }`, "must be a typed Go pointer"},
		{"slice noninteger length", `function value(pointer: *byte): byte[] { return danger.Slice(pointer, 1.5); }`, "length must be an integer"},
		{"slice negative length", `function value(pointer: *byte): byte[] { return danger.Slice(pointer, -1); }`, "length cannot be negative"},
		{"slice data fixed array", `function value(items: [2]byte): *byte { return danger.SliceData(items); }`, "argument must be a slice"},
		{"slice data string", `function value(): *byte { return danger.SliceData("x"); }`, "argument must be a slice"},
		{"string wrong pointer", `function value(pointer: *int): string { return danger.String(pointer, 1); }`, "pointer must be *byte"},
		{"string noninteger length", `function value(pointer: *byte): string { return danger.String(pointer, false); }`, "length must be an integer"},
		{"string negative length", `function value(pointer: *byte): string { return danger.String(pointer, -1); }`, "length cannot be negative"},
		{"string length out of range", `function value(pointer: *byte): string { return danger.String(pointer, 999999999999999999999999999999); }`, "length is out of range"},
		{"string data nonstring", `function value(): *byte { return danger.StringData(1); }`, "argument must be a string"},
		{"builtin is not first class", `function value(): int { const operation = danger.Sizeof; return 1; }`, "is not supported"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := `import go danger from "unsafe"; ` + test.source
			diagnostics := checkSourceWithPolicy(t, source, GoInteropPolicy{AllowUnsafe: true})
			if joined := strings.Join(diagnostics, "\n"); !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics=%v, want %q", diagnostics, test.want)
			}
		})
	}
	validNil := `import go danger from "unsafe";
function addNil(): danger.Pointer { return danger.Add(nil, 0); }
function emptyText(): string { return danger.String(nil, 0); }`
	if diagnostics := checkSourceWithPolicy(t, validNil, GoInteropPolicy{AllowUnsafe: true}); len(diagnostics) != 0 {
		t.Fatalf("valid nil diagnostics=%v", diagnostics)
	}
}

func TestGoVariadicIndividualArgumentsMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"zero", `import go path from "path"; function value(): string { return path.Join(); }`, ""},
		{"one", `import go path from "path"; function value(): string { return path.Join("a"); }`, ""},
		{"many", `import go path from "path"; function value(): string { return path.Join("a", "b", "c"); }`, ""},
		{"any elements", `import go fmt from "fmt"; function value(): string { return fmt.Sprintf("%s:%d", "item", 2); }`, ""},
		{"wrong element type", `import go path from "path"; function value(): string { return path.Join("a", 1); }`, "cannot use integer literal as string"},
		{"spread slice", `import go path from "path"; function value(parts: string[]): string { return path.Join(parts...); }`, ""},
		{"spread literal", `import go path from "path"; function value(): string { return path.Join(["a", "b"]...); }`, ""},
		{"spread non-variadic", `import go strings from "strings"; function value(parts: string[]): string { return strings.ToUpper(parts...); }`, "is not variadic"},
		{"spread requires slice", `import go path from "path"; function value(): string { return path.Join("a"...); }`, "cannot use string as string[]"},
		{"spread element mismatch", `import go path from "path"; function value(parts: int[]): string { return path.Join(parts...); }`, "cannot use int[] as string[]"},
		{"spread cannot mix individuals", `import go path from "path"; function value(parts: string[]): string { return path.Join("a", parts...); }`, "expects 1 arguments (0 fixed and one slice), got 2"},
		{"spread builtin conversion", `function value(parts: int[]): int { return int(parts...); }`, "spread arguments cannot be used in type conversions"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			joined := strings.Join(diagnostics, "\n")
			if test.want == "" && len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
			if test.want != "" && !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestGoCallbackAndFunctionValueMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"arrow callback", `import go strings from "strings"; function value(): string { return strings.Map((item: int32): int32 => item + 1, "ab"); }`, ""},
		{"capturing callback", `import go sort from "sort"; function value(): int { const items = [3, 1, 2]; sort.Slice(items, (left: int, right: int): boolean => items[left] < items[right]); return items[0]; }`, ""},
		{"named function callback", `import go strings from "strings"; function identity(item: int32): int32 { return item; } function value(): string { return strings.Map(identity, "ab"); }`, ""},
		{"package function value", `import go strings from "strings"; function value(): string { const upper = strings.ToUpper; return upper("ab"); }`, ""},
		{"bound method value", `import go strings from "strings"; function value(): string { const replace = strings.NewReplacer("a", "b").Replace; return replace("a"); }`, ""},
		{"nil callback", `import go time from "time"; function value(): *time.Timer { return time.AfterFunc(time.Second, nil); }`, ""},
		{"callback parameter count", `import go strings from "strings"; function value(): string { return strings.Map((item: int32, extra: int32): int32 => item, "ab"); }`, "cannot use (int32, int32) => int32 as (int32) => int32"},
		{"callback parameter type", `import go strings from "strings"; function value(): string { return strings.Map((item: int): int32 => int32(item), "ab"); }`, "cannot use (int) => int32 as (int32) => int32"},
		{"callback result type", `import go strings from "strings"; function value(): string { return strings.Map((item: int32): string => "x", "ab"); }`, "cannot use (int32) => string as (int32) => int32"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			joined := strings.Join(diagnostics, "\n")
			if test.want == "" && len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
			if test.want != "" && !strings.Contains(joined, test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestGoInterfaceValueAndExplicitClassImplementationMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"concrete pointer to interface", `import go io from "io"; import go strings from "strings"; function consume(reader: io.Reader): int64 { const [count, err] = io.Copy(io.Discard, reader); return count; } function value(): int64 { return consume(strings.NewReader("abc")); }`},
		{"interface result and method", `import go io from "io"; import go strings from "strings"; function value(): error { const reader = io.NopCloser(strings.NewReader("abc")); return reader.Close(); }`},
		{"typed nil interface", `import go io from "io"; function value(): boolean { let reader: io.Reader = nil; return reader == nil; }`},
		{"class to empty interface", `import go json from "encoding/json"; class Value {} function encode(): Result<byte[]> { const data = json.Marshal(new Value())?; return ok(data); }`},
		{"generic class to empty interface", `import go json from "encoding/json"; class Value<T> { constructor(public item: T) {} } function encode(value: string): Result<byte[]> { const data = json.Marshal(new Value<string>(value))?; return ok(data); }`},
		{"explicit class implementation", `import go sort from "sort"; class Numbers implements sort.Interface { constructor(private values: int[]) {} public function len(): int { return this.values[0] * 0 + 3; } public function less(left: int, right: int): boolean { return this.values[left] < this.values[right]; } public function swap(left: int, right: int): void { const saved = this.values[left]; this.values[left] = this.values[right]; this.values[right] = saved; } } function value(values: int[]): void { sort.Sort(new Numbers(values)); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}

	failures := []struct {
		name   string
		source string
		want   string
	}{
		{"non-interface target", `import go time from "time"; class Value implements time.Duration {}`, "is not an interface"},
		{"missing methods", `import go sort from "sort"; class Value implements sort.Interface {}`, "missing exported method"},
		{"wrong method signature", `import go sort from "sort"; class Value implements sort.Interface { public function len(): string { return ""; } public function less(left: int, right: int): boolean { return false; } public function swap(left: int, right: int): void {} }`, "method Len has () => string"},
		{"static method", `import go sort from "sort"; class Value implements sort.Interface { public static function len(): int { return 0; } public function less(left: int, right: int): boolean { return false; } public function swap(left: int, right: int): void {} }`, "method Len cannot be static"},
		{"duplicate interface", `import go sort from "sort"; class Value implements sort.Interface, sort.Interface { public function len(): int { return 0; } public function less(left: int, right: int): boolean { return false; } public function swap(left: int, right: int): void {} }`, "duplicate implemented Go interface"},
		{"implicit implementation rejected", `import go sort from "sort"; class Value { public function len(): int { return 0; } public function less(left: int, right: int): boolean { return false; } public function swap(left: int, right: int): void {} } function use(): void { sort.Sort(new Value()); }`, "cannot use Value as sort.Interface"},
	}
	for _, test := range failures {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestGoGenericInferenceMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"contains inferred element", `import go slices from "slices"; function value(items: int[]): boolean { return slices.Contains(items, 2); }`},
		{"clone preserves slice", `import go slices from "slices"; function value(items: int[]): int[] { return slices.Clone(items); }`},
		{"callback participates in inference", `import go slices from "slices"; function value(items: int[]): int { return slices.IndexFunc(items, (item: int): boolean => item > 2); }`},
		{"map key and value inference", `import go maps from "maps"; function value(items: Map<string, int>): Map<string, int> { return maps.Clone(items); }`},
		{"generic variadic spread", `import go slices from "slices"; function value(items: int[][]): int[] { return slices.Concat(items...); }`},
		{"explicit slice type", `import go slices from "slices"; function value(items: int[]): int[] { return slices.Clone[int[]](items); }`},
		{"partial explicit arguments infer remainder", `import go slices from "slices"; function value(): int[] { return slices.Concat[int[]](); }`},
		{"multiple explicit arguments", `import go maps from "maps"; function value(items: Map<string, int>): Map<string, int> { return maps.Clone[Map<string, int>, string, int](items); }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}

	failures := []struct {
		name   string
		source string
		want   string
	}{
		{"inconsistent element", `import go slices from "slices"; function value(items: int[]): boolean { return slices.Contains(items, "x"); }`, "cannot infer Go type arguments"},
		{"comparable constraint", `import go slices from "slices"; function value(items: Map<string, int>[], candidate: Map<string, int>): boolean { return slices.Contains(items, candidate); }`, "does not satisfy comparable"},
		{"no inference evidence", `import go slices from "slices"; function value(): int[] { return slices.Concat(); }`, "cannot infer"},
		{"callback mismatch", `import go slices from "slices"; function value(items: int[]): int { return slices.IndexFunc(items, (item: string): boolean => true); }`, "cannot infer Go type arguments"},
		{"too many explicit arguments", `import go slices from "slices"; function value(items: int[]): int[] { return slices.Clone[int[], int, string](items); }`, "has 2 Go type parameters, got 3 explicit type arguments"},
		{"explicit constraint mismatch", `import go slices from "slices"; function value(items: Map<string, int>[]): boolean { return slices.Contains[Map<string, int>[], Map<string, int>](items, items[0]); }`, "does not satisfy comparable"},
		{"explicit argument value mismatch", `import go slices from "slices"; function value(items: string[]): int[] { return slices.Clone[int[], int](items); }`, "cannot apply explicit Go type arguments"},
		{"type arguments on non-generic function", `import go strings from "strings"; function value(): string { return strings.ToUpper[string]("x"); }`, "is not a generic Go function"},
	}
	for _, test := range failures {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestGoGenericNamedTypeAndMethodMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"atomic pointer annotation and methods", `import go atomic from "sync/atomic"; function value(input: string): string { let pointer: atomic.Pointer<string> = atomic.Pointer<string>{}; pointer.Store(&input); return *pointer.Load(); }`},
		{"generic result inferred", `import go unique from "unique"; function value(input: string): string { const handle = unique.Make(input); return handle.Value(); }`},
		{"generic result explicit", `import go unique from "unique"; function value(input: string): string { const handle: unique.Handle<string> = unique.Make(input); return handle.Value(); }`},
		{"generic named type in collection", `import go unique from "unique"; function value(input: string): unique.Handle<string>[] { return [unique.Make(input)]; }`},
		{"cross package type argument", `import go atomic from "sync/atomic"; import go time from "time"; function value(): atomic.Pointer<time.Duration> { return atomic.Pointer<time.Duration>{}; }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}

	failures := []struct {
		name   string
		source string
		want   string
	}{
		{"missing type argument", `import go atomic from "sync/atomic"; function value(): atomic.Pointer { return atomic.Pointer{}; }`, "requires 1 type arguments"},
		{"too many type arguments", `import go atomic from "sync/atomic"; function value(): atomic.Pointer<string, int> { return atomic.Pointer<string, int>{}; }`, "expects 1 type arguments, got 2"},
		{"constraint mismatch", `import go unique from "unique"; function value(): unique.Handle<Map<string, int>> { return unique.Handle<Map<string, int>>{}; }`, "does not satisfy comparable"},
		{"non generic type arguments", `import go time from "time"; function value(): time.Duration<string> { return 0; }`, "Go type time.Duration is not generic"},
		{"unknown type argument", `import go atomic from "sync/atomic"; function value(): atomic.Pointer<Missing> { return atomic.Pointer<Missing>{}; }`, "unknown type"},
	}
	for _, test := range failures {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestGoTypeAssertionMatrix(t *testing.T) {
	success := []struct {
		name   string
		source string
	}{
		{"unchecked concrete assertion", `import go io from "io"; import go strings from "strings"; function value(reader: io.Reader): *strings.Reader { return reader as! *strings.Reader; }`},
		{"checked concrete assertion", `import go io from "io"; import go strings from "strings"; function value(reader: io.Reader): boolean { const [typed, ok] = reader as? *strings.Reader; return ok; }`},
		{"checked interface assertion", `import go io from "io"; function value(reader: io.Reader): boolean { const [writer, ok] = reader as? io.Writer; return ok; }`},
		{"unchecked interface assertion", `import go io from "io"; function value(reader: io.Reader): io.Writer { return reader as! io.Writer; }`},
		{"error assertion", `import go os from "os"; function value(err: error): *os.PathError { return err as! *os.PathError; }`},
		{"assertion in comparison", `import go io from "io"; import go strings from "strings"; function value(reader: io.Reader): boolean { return (reader as! *strings.Reader) != nil; }`},
	}
	for _, test := range success {
		t.Run(test.name, func(t *testing.T) {
			if diagnostics := checkSource(t, test.source); len(diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diagnostics)
			}
		})
	}

	failures := []struct {
		name   string
		source string
		want   string
	}{
		{"non interface source", `function value(input: int): string { return input as! string; }`, "requires a Go interface value"},
		{"impossible concrete assertion", `import go io from "io"; function value(input: io.Reader): int { return input as! int; }`, "cannot contain asserted type int"},
		{"checked assertion requires destructuring", `import go io from "io"; import go strings from "strings"; function value(input: io.Reader): *strings.Reader { return input as? *strings.Reader; }`, "require destructuring"},
		{"unchecked assertion is not multi value", `import go io from "io"; import go strings from "strings"; function value(input: io.Reader): boolean { const [typed, ok] = input as! *strings.Reader; return ok; }`, "requires a multiple-return value"},
		{"class assertion target", `import go io from "io"; class Value {} function value(input: io.Reader): boolean { const [typed, ok] = input as? Value; return ok; }`, "cannot be represented as a Go type"},
	}
	for _, test := range failures {
		t.Run(test.name, func(t *testing.T) {
			diagnostics := checkSource(t, test.source)
			if !strings.Contains(strings.Join(diagnostics, "\n"), test.want) {
				t.Fatalf("diagnostics = %v, want %q", diagnostics, test.want)
			}
		})
	}
}

func TestGoMultipleResultFailureMatrix(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"too many bindings", `import go strconv from "strconv"; function value(): int { const [parsed, err, extra] = strconv.Atoi("1"); return parsed; }`, "got 3 bindings for 2 results"},
		{"too few bindings", `import go runtime from "runtime"; function value(): int { const [pc, file] = runtime.Caller(0); return 1; }`, "got 2 bindings for 4 results"},
		{"void rhs", `import go runtime from "runtime"; function value(): int { const [first, second] = runtime.GC(); return 1; }`, "got void"},
		{"single result rhs", `import go strconv from "strconv"; function value(): int { const [text, other] = strconv.Itoa(1); return 1; }`, "requires a multiple-return value"},
		{"single value inference", `import go strconv from "strconv"; function value(): int { const result = strconv.Atoi("1"); return 1; }`, "require destructuring"},
		{"single value argument", `import go strconv from "strconv"; function identity(value: int): int { return value; } function value(): int { return identity(strconv.Atoi("1")); }`, "require destructuring"},
		{"single value array", `import go strconv from "strconv"; function value(): int { const items = [strconv.Atoi("1")]; return 1; }`, "require destructuring"},
		{"single value object", `import go strconv from "strconv"; function value(): int { const item = { parsed: strconv.Atoi("1") }; return 1; }`, "require destructuring"},
		{"single value arrow", `import go strconv from "strconv"; function value(): int { const parse = () => strconv.Atoi("1"); return 1; }`, "require destructuring"},
		{"single value binary", `import go strconv from "strconv"; function value(): int { return strconv.Atoi("1") + 1; }`, "require destructuring"},
		{"duplicate binding", `import go strconv from "strconv"; function value(): int { const [parsed, parsed] = strconv.Atoi("1"); return parsed; }`, "duplicate local name"},
		{"assign const", `import go strconv from "strconv"; function value(): int { const parsed = 0; let err: error = nil; [parsed, err] = strconv.Atoi("1"); return parsed; }`, "cannot assign to const"},
		{"assign undefined", `import go strconv from "strconv"; function value(): int { let err: error = nil; [missing, err] = strconv.Atoi("1"); return 1; }`, "undefined name"},
		{"assignment type mismatch", `import go strconv from "strconv"; function value(): int { let parsed: string = ""; let err: error = nil; [parsed, err] = strconv.Atoi("1"); return 1; }`, "cannot use int as string"},
		{"error target mismatch", `import go strconv from "strconv"; function value(): int { let parsed = 0; let err: string = ""; [parsed, err] = strconv.Atoi("1"); return parsed; }`, "cannot use error as string"},
		{"assignment count mismatch", `import go strconv from "strconv"; function value(): int { let parsed = 0; let err: error = nil; let extra = 0; [parsed, err, extra] = strconv.Atoi("1"); return parsed; }`, "got 3 bindings for 2 results"},
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
