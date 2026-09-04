package codegen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	kinmokuseiAST "github.com/puffball1567/kinmokusei/internal/ast"
	"github.com/puffball1567/kinmokusei/internal/lexer"
	"github.com/puffball1567/kinmokusei/internal/parser"
	"github.com/puffball1567/kinmokusei/internal/sema"
)

func checkedCABIProgram(t *testing.T, source string) CABIOutput {
	t.Helper()
	tokens, lexDiagnostics := lexer.Lex("cabi.km", source)
	if len(lexDiagnostics) != 0 {
		t.Fatalf("lexer diagnostics=%v", lexDiagnostics)
	}
	program, parseDiagnostics := parser.Parse(tokens)
	if len(parseDiagnostics) != 0 {
		t.Fatalf("parser diagnostics=%v", parseDiagnostics)
	}
	if diagnostics := sema.Check(program); len(diagnostics) != 0 {
		t.Fatalf("semantic diagnostics=%v", diagnostics)
	}
	output, err := GenerateCABI(program)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func TestGenerateCABIScalarAndStatusMatrix(t *testing.T) {
	output := checkedCABIProgram(t, `
export c("kinmokusei_ping") function ping(): void {}
export c("kinmokusei_scalar") function scalar(a: byte, b: int32, c: int64, d: float32, e: float, f: uint16, g: uint32, h: uint64): uint64 { return h + uint64(f) + uint64(g); }
export c("kinmokusei_not") function logicalNot(value: boolean): boolean { return !value; }
`)
	for _, want := range []string{
		"//export kinmokusei_ping", "func kinmokusei_ping() (__kinmokuseiStatus C.int32_t)",
		"//export kinmokusei_scalar", "arg0 C.uint8_t", "arg1 C.int32_t", "arg2 C.int64_t", "arg3 C.float", "arg4 C.double", "arg5 C.uint16_t", "arg6 C.uint32_t", "arg7 C.uint64_t",
		"outResult *C.uint64_t", "if outResult == nil", "if recover() != nil", "__kinmokuseiStatus = C.int32_t(1)",
		"__kinmokuseiResult := scalar(byte(arg0), int32(arg1), int64(arg2), float32(arg3), float64(arg4), uint16(arg5), uint32(arg6), uint64(arg7))",
		"//export kinmokusei_not", "func kinmokusei_not(arg0 C.uint8_t, outResult *C.uint8_t)", "__kinmokuseiResult := logicalNot(arg0 != 0)",
		"*outResult = C.uint8_t(0)", "if __kinmokuseiResult", "*outResult = C.uint8_t(1)",
		"func main() {}",
	} {
		if !strings.Contains(string(output.Gateway), want) {
			t.Errorf("wrapper missing %q:\n%s", want, output.Gateway)
		}
	}
	for _, want := range []string{
		"#define KINMOKUSEI_ABI_OK ((int32_t)0)", "#define KINMOKUSEI_ABI_PANIC ((int32_t)1)",
		"#define KINMOKUSEI_ABI_INVALID_ARGUMENT ((int32_t)2)",
		"#define KINMOKUSEI_ABI_GATEWAY_VERSION_MAJOR 1", "#define KINMOKUSEI_ABI_GATEWAY_VERSION_MINOR 0",
		`#define KINMOKUSEI_ABI_FINGERPRINT "` + output.Fingerprint + `"`,
		"int32_t kinmokusei_ping(void);",
		"int32_t kinmokusei_not(uint8_t arg0, uint8_t *out_result);",
		"int32_t kinmokusei_scalar(uint8_t arg0, int32_t arg1, int64_t arg2, float arg3, double arg4, uint16_t arg5, uint32_t arg6, uint64_t arg7, uint64_t *out_result);",
	} {
		if !strings.Contains(string(output.Header), want) {
			t.Errorf("header missing %q:\n%s", want, output.Header)
		}
	}
}

func TestGenerateCABIFixedWidthEnumTransport(t *testing.T) {
	output := checkedCABIProgram(t, `
type CodeBase = distinct uint32;
enum Level: int16 { Minimum = -32768, Normal = 4, Maximum = 32767 }
enum Code: CodeBase { Empty, Ready = 41 }
export c("kinmokusei_level") function level(value: Level): Level { return value; }
export c("kinmokusei_code") function code(value: Code): Code { return value; }
`)
	for _, want := range []string{
		"func kinmokusei_level(arg0 C.int16_t, outResult *C.int16_t)",
		"__kinmokuseiResult := level(Level(arg0))",
		"*outResult = C.int16_t(__kinmokuseiResult)",
		"func kinmokusei_code(arg0 C.uint32_t, outResult *C.uint32_t)",
		"__kinmokuseiResult := code(Code(arg0))",
		"*outResult = C.uint32_t(__kinmokuseiResult)",
	} {
		if !strings.Contains(string(output.Gateway), want) {
			t.Errorf("enum wrapper missing %q:\n%s", want, output.Gateway)
		}
	}
	for _, want := range []string{
		"int32_t kinmokusei_level(int16_t arg0, int16_t *out_result);",
		"int32_t kinmokusei_code(uint32_t arg0, uint32_t *out_result);",
	} {
		if !strings.Contains(string(output.Header), want) {
			t.Errorf("enum header missing %q:\n%s", want, output.Header)
		}
	}
	var manifest cabiManifest
	if err := json.Unmarshal(output.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Functions) != 2 || manifest.Functions[0].Parameters[0] != "uint32_t" || manifest.Functions[0].Result != "uint32_t" || manifest.Functions[1].Parameters[0] != "int16_t" || manifest.Functions[1].Result != "int16_t" {
		t.Fatalf("enum manifest functions=%#v", manifest.Functions)
	}
}

func TestGenerateCABIExportListFunctionsAndArrows(t *testing.T) {
	output := checkedCABIProgram(t, `
function add(left: int32, right: int32): int32 { return left + right; }
const sub = (left: int32, right: int32): int32 => left - right;
export c(
  "kinmokusei_add",
  "kinmokusei_sub",
) {
  add,
  sub,
};
`)
	for _, want := range []string{
		"//export kinmokusei_add",
		"__kinmokuseiResult := add(int32(arg0), int32(arg1))",
		"//export kinmokusei_sub",
		"__kinmokuseiResult := sub(int32(arg0), int32(arg1))",
	} {
		if !strings.Contains(string(output.Gateway), want) {
			t.Errorf("gateway missing %q:\n%s", want, output.Gateway)
		}
	}
	for _, want := range []string{
		"int32_t kinmokusei_add(int32_t arg0, int32_t arg1, int32_t *out_result);",
		"int32_t kinmokusei_sub(int32_t arg0, int32_t arg1, int32_t *out_result);",
	} {
		if !strings.Contains(string(output.Header), want) {
			t.Errorf("header missing %q:\n%s", want, output.Header)
		}
	}
	var manifest cabiManifest
	if err := json.Unmarshal(output.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Functions) != 2 || manifest.Functions[0].Symbol != "kinmokusei_add" || manifest.Functions[1].Symbol != "kinmokusei_sub" {
		t.Fatalf("manifest functions=%#v", manifest.Functions)
	}
}

func TestCABIExportListArtifactOrderIsSymbolCanonical(t *testing.T) {
	listed := checkedCABIProgram(t, `
function zebra(value: int64): void {}
const alpha = (value: int32): int32 => value;
export c("kinmokusei_zebra", "kinmokusei_alpha") {zebra, alpha};
`)
	inline := checkedCABIProgram(t, `
export c("kinmokusei_alpha") function alpha(value: int32): int32 { return value; }
export c("kinmokusei_zebra") function zebra(value: int64): void {}
`)
	if !bytes.Equal(listed.Header, inline.Header) || !bytes.Equal(listed.Manifest, inline.Manifest) || listed.Fingerprint != inline.Fingerprint {
		t.Fatalf("equivalent list and inline ABI differ:\nlist=%s\ninline=%s", listed.Manifest, inline.Manifest)
	}
}

func TestGenerateCABIRejectsMissingExports(t *testing.T) {
	tokens, _ := lexer.Lex("none.km", `function value(): int32 { return 1; }`)
	program, _ := parser.Parse(tokens)
	if _, err := GenerateCABI(program); err == nil || !strings.Contains(err.Error(), "does not declare") {
		t.Fatalf("err=%v", err)
	}
}

func TestGenerateCABIIsDeterministic(t *testing.T) {
	source := `export c("kinmokusei_value") function value(input: int32): int64 { return int64(input); }`
	first := checkedCABIProgram(t, source)
	second := checkedCABIProgram(t, source)
	if !bytes.Equal(first.Gateway, second.Gateway) || !bytes.Equal(first.Header, second.Header) || !bytes.Equal(first.Manifest, second.Manifest) || first.Fingerprint != second.Fingerprint {
		t.Fatalf("C ABI generation is not deterministic:\n--- first gateway ---\n%s\n--- second gateway ---\n%s\n--- first header ---\n%s\n--- second header ---\n%s\n--- first manifest ---\n%s\n--- second manifest ---\n%s", first.Gateway, second.Gateway, first.Header, second.Header, first.Manifest, second.Manifest)
	}
}

func TestCABIManifestCanonicalOrderAndFingerprintMatrix(t *testing.T) {
	first := checkedCABIProgram(t, `
export c("kinmokusei_zebra") function zebra(value: int64): void {}
export c("kinmokusei_alpha") function alpha(value: int32): int32 { return value; }
`)
	second := checkedCABIProgram(t, `
export c("kinmokusei_alpha") function alpha(value: int32): int32 { return value; }
export c("kinmokusei_zebra") function zebra(value: int64): void {}
`)
	if !bytes.Equal(first.Gateway, second.Gateway) || !bytes.Equal(first.Header, second.Header) || !bytes.Equal(first.Manifest, second.Manifest) || first.Fingerprint != second.Fingerprint {
		t.Fatalf("declaration order changed ABI artifacts:\nfirst=%s\nsecond=%s", first.Manifest, second.Manifest)
	}
	var manifest cabiManifest
	if err := json.Unmarshal(first.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != 1 || manifest.GatewayVersion != (cabiManifestVersion{Major: 1, Minor: 0}) || manifest.Fingerprint != first.Fingerprint {
		t.Fatalf("manifest metadata=%#v fingerprint=%q", manifest, first.Fingerprint)
	}
	canonical, err := json.Marshal(cabiFingerprintInput{
		SchemaVersion: manifest.SchemaVersion, GatewayVersion: manifest.GatewayVersion, Status: manifest.Status, Functions: manifest.Functions,
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	wantFingerprint := "sha256:" + hex.EncodeToString(digest[:])
	if manifest.Fingerprint != wantFingerprint {
		t.Fatalf("fingerprint=%q want=%q canonical=%s", manifest.Fingerprint, wantFingerprint, canonical)
	}
	if len(manifest.Functions) != 2 || manifest.Functions[0].Symbol != "kinmokusei_alpha" || manifest.Functions[0].Result != "int32_t" || manifest.Functions[0].ResultTransport != "out_parameter" || manifest.Functions[1].Symbol != "kinmokusei_zebra" || manifest.Functions[1].Result != "void" || manifest.Functions[1].ResultTransport != "none" {
		t.Fatalf("functions=%#v", manifest.Functions)
	}
	changedType := checkedCABIProgram(t, `export c("kinmokusei_alpha") function alpha(value: int64): int32 { return int32(value); }`)
	changedSymbol := checkedCABIProgram(t, `export c("kinmokusei_beta") function alpha(value: int32): int32 { return value; }`)
	if changedType.Fingerprint == first.Fingerprint || changedSymbol.Fingerprint == first.Fingerprint || changedType.Fingerprint == changedSymbol.Fingerprint {
		t.Fatalf("ABI changes did not change fingerprint: base=%q type=%q symbol=%q", first.Fingerprint, changedType.Fingerprint, changedSymbol.Fingerprint)
	}
}

func TestMapCABITypeMatrix(t *testing.T) {
	integer := kinmokuseiAST.TypeRef{Name: "int32"}
	tests := []struct {
		name      string
		ref       kinmokuseiAST.TypeRef
		allowVoid bool
		want      cabiScalar
		valid     bool
	}{
		{"byte", kinmokuseiAST.TypeRef{Name: "byte"}, false, cabiScalar{"uint8_t", "C.uint8_t", "byte"}, true},
		{"uint8 alias", kinmokuseiAST.TypeRef{Name: "uint8"}, false, cabiScalar{"uint8_t", "C.uint8_t", "uint8"}, true},
		{"boolean", kinmokuseiAST.TypeRef{Name: "boolean"}, false, cabiScalar{"uint8_t", "C.uint8_t", "bool"}, true},
		{"int8", kinmokuseiAST.TypeRef{Name: "int8"}, false, cabiScalar{"int8_t", "C.int8_t", "int8"}, true},
		{"int16", kinmokuseiAST.TypeRef{Name: "int16"}, false, cabiScalar{"int16_t", "C.int16_t", "int16"}, true},
		{"int32", kinmokuseiAST.TypeRef{Name: "int32"}, false, cabiScalar{"int32_t", "C.int32_t", "int32"}, true},
		{"int64", kinmokuseiAST.TypeRef{Name: "int64"}, false, cabiScalar{"int64_t", "C.int64_t", "int64"}, true},
		{"uint16", kinmokuseiAST.TypeRef{Name: "uint16"}, false, cabiScalar{"uint16_t", "C.uint16_t", "uint16"}, true},
		{"uint32", kinmokuseiAST.TypeRef{Name: "uint32"}, false, cabiScalar{"uint32_t", "C.uint32_t", "uint32"}, true},
		{"uint64", kinmokuseiAST.TypeRef{Name: "uint64"}, false, cabiScalar{"uint64_t", "C.uint64_t", "uint64"}, true},
		{"float32", kinmokuseiAST.TypeRef{Name: "float32"}, false, cabiScalar{"float", "C.float", "float32"}, true},
		{"float", kinmokuseiAST.TypeRef{Name: "float"}, false, cabiScalar{"double", "C.double", "float64"}, true},
		{"float64", kinmokuseiAST.TypeRef{Name: "float64"}, false, cabiScalar{"double", "C.double", "float64"}, true},
		{"number", kinmokuseiAST.TypeRef{Name: "number"}, false, cabiScalar{"double", "C.double", "float64"}, true},
		{"void result", kinmokuseiAST.TypeRef{Name: "void"}, true, cabiScalar{goType: "void"}, true},
		{"void parameter", kinmokuseiAST.TypeRef{Name: "void"}, false, cabiScalar{}, false},
		{"nullable boolean", kinmokuseiAST.TypeRef{Name: "boolean", Nullable: true}, false, cabiScalar{}, false},
		{"machine int", kinmokuseiAST.TypeRef{Name: "int"}, false, cabiScalar{}, false},
		{"machine uint", kinmokuseiAST.TypeRef{Name: "uint"}, false, cabiScalar{}, false},
		{"string", kinmokuseiAST.TypeRef{Name: "string"}, false, cabiScalar{}, false},
		{"slice", kinmokuseiAST.TypeRef{Element: &integer}, false, cabiScalar{}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := mapCABIType(test.ref, test.allowVoid)
			if valid != test.valid || got != test.want {
				t.Fatalf("mapCABIType()=(%#v, %v), want (%#v, %v)", got, valid, test.want, test.valid)
			}
		})
	}
}
