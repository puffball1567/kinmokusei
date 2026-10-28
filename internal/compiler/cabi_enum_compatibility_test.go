package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puffball1567/kinmokusei/internal/cabi"
)

func TestCABIEnumAndSemanticCompatibility(t *testing.T) {
	enumSource := func(members, parameter, result string) string {
		return fmt.Sprintf(`enum Mode: int32 { %s }
function open(mode: %s): %s { return %s(0); }
export c("k_open") {open};`, members, parameter, result, result)
	}
	base := enumSource("Read = 0, Write = 1", "Mode", "Mode")
	tests := []struct {
		name, baseline, current string
		compatible, exact       bool
	}{
		{"renumber", base, enumSource("Write = 0, Read = 1", "Mode", "Mode"), false, false},
		{"remove member", base, enumSource("Read = 0", "Mode", "Mode"), false, false},
		{"rename member", base, enumSource("Read = 0, Save = 1", "Mode", "Mode"), false, false},
		{"add member", base, enumSource("Read = 0, Write = 1, Append = 2", "Mode", "Mode"), true, false},
		{"declaration order", base, enumSource("Write = 1, Read = 0", "Mode", "Mode"), true, true},
		{"implicit renumber", enumSource("Read, Write", "Mode", "Mode"), enumSource("Write, Read", "Mode", "Mode"), false, false},
		{"enum to integer parameter", base, enumSource("Read = 0, Write = 1", "int32", "Mode"), false, false},
		{"enum to integer result", base, enumSource("Read = 0, Write = 1", "Mode", "int32"), false, false},
		{"boolean parameter", `export c("k_open") function open(v: boolean): void {}`, `export c("k_open") function open(v: byte): void {}`, false, false},
		{"boolean result", `export c("k_open") function open(): boolean { return false; }`, `export c("k_open") function open(): byte { return 0; }`, false, false},
		{"byte alias", `export c("k_open") function open(v: byte): byte { return v; }`, `export c("k_open") function open(v: uint8): uint8 { return v; }`, true, true},
		{"float alias", `export c("k_open") function open(v: float): float { return v; }`, `export c("k_open") function open(v: number): float64 { return v; }`, true, true},
		{"enum name", base, strings.ReplaceAll(base, "Mode", "Renamed"), true, true},
		{"resolved constants", base, enumSource("Write = 0x1, Read = 2 - 2", "Mode", "Mode"), true, true},
		{"unreachable enum", base, base + ` enum Private: int8 { Hidden = 7 }`, true, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			baseline := emitCABIManifestForTest(t, test.baseline)
			current := emitCABIManifestForTest(t, test.current)
			got, err := CompareCABIManifests(baseline, current)
			if err != nil {
				t.Fatal(err)
			}
			if got.Compatible() != test.compatible || got.ExactFingerprint != test.exact {
				t.Fatalf("compatibility=%#v; want compatible=%v exact=%v", got, test.compatible, test.exact)
			}
			if !test.compatible && !strings.Contains(strings.Join(got.BreakingChanges, "\n"), "k_open") {
				t.Fatalf("missing affected symbol: %#v", got)
			}
			if test.name == "add member" && len(got.TypeAdditions) != 2 {
				t.Fatalf("missing parameter/result enum additions: %#v", got)
			}
		})
	}
}

func TestCABIEnumContractIntegerLimits(t *testing.T) {
	for _, test := range []struct{ underlying, members string }{
		{"int8", "Min = -128, Max = 127"},
		{"uint8", "Min = 0, Max = 255"},
		{"int16", "Min = -32768, Max = 32767"},
		{"uint16", "Min = 0, Max = 65535"},
		{"int32", "Min = -2147483648, Max = 2147483647"},
		{"uint32", "Min = 0, Max = 4294967295"},
		{"int64", "Min = -9223372036854775808, Max = 9223372036854775807"},
		{"uint64", "Min = 0, Max = 18446744073709551615"},
	} {
		t.Run(test.underlying, func(t *testing.T) {
			source := fmt.Sprintf(`type Base = distinct %s; enum E: Base { %s }
export c("k_echo") function echo(v: E): E { return v; }`, test.underlying, test.members)
			manifest := emitCABIManifestForTest(t, source)
			if result, err := CompareCABIManifests(manifest, manifest); err != nil || !result.ExactFingerprint {
				t.Fatalf("result=%#v err=%v", result, err)
			}
			_, maximum, _ := strings.Cut(test.members, "Max = ")
			if !strings.Contains(string(manifest), `"value": "`+maximum+`"`) {
				t.Fatalf("maximum value was not recorded exactly: %s", manifest)
			}
		})
	}
}

func TestCABIImportedEnumContract(t *testing.T) {
	base := emitCABIManifestForTest(t, `enum Mode: int32 { Read = 0, Write = 1 }
export c("k_open") function open(v: Mode): Mode { return v; }`)
	for _, members := range []string{"Read = 0, Write = 1", "Read = 1, Write = 0"} {
		directory := t.TempDir()
		entry := filepath.Join(directory, "main.km")
		for name, source := range map[string]string{
			"main.km": `enum Mode: uint16 { Local = 7 }
import { open } from "./library"; export c("k_open") {open};`,
			"library.km": fmt.Sprintf(`enum Mode: int32 { %s }
function open(v: Mode): Mode { return v; }`, members),
		} {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		artifacts, diagnostics, err := EmitCABI([]string{entry})
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
		}
		result, err := CompareCABIManifests(base, artifacts.Manifest)
		wantExact := members == "Read = 0, Write = 1"
		if err != nil || result.ExactFingerprint != wantExact || result.Compatible() != wantExact {
			t.Fatalf("result=%#v err=%v wantExact=%v", result, err, wantExact)
		}
	}
}

func TestCABIRejectsMalformedTypeContracts(t *testing.T) {
	valid := emitCABIManifestForTest(t, `enum E: int8 { A = 0, B = 1 }
export c("k_echo") function echo(v: E): E { return v; }`)
	tests := []struct {
		name, want string
		mutate     func(*cabiCompatibilityManifest)
	}{
		{"missing parameters", "parameterTypes", func(m *cabiCompatibilityManifest) { m.Functions[0].ParameterTypes = nil }},
		{"wrong arity", "parameterTypes", func(m *cabiCompatibilityManifest) {
			m.Functions[0].ParameterTypes = append(m.Functions[0].ParameterTypes, cabi.Type{Kind: "integer"})
		}},
		{"missing result", "semantic type", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType = cabi.Type{} }},
		{"wrong kind", "semantic type", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Kind = "unknown" }},
		{"wrong transport", "semantic type", func(m *cabiCompatibilityManifest) { m.Functions[0].Parameters[0] = "double" }},
		{"wrong result mode", "resultTransport", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultTransport = "none" }},
		{"empty enum", "at least one", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members = nil }},
		{"non-enum members", "only enum", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Kind = "integer" }},
		{"duplicate member", "unique names", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[1].Name = "A" }},
		{"member order", "ascending order", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[0].Name = "Z" }},
		{"empty member", "nonempty", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[0].Name = "" }},
		{"noncanonical value", "canonical decimal", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[0].Value = "00" }},
		{"not integer", "canonical decimal", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[0].Value = "0.5" }},
		{"overflow", "outside int8_t", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[0].Value = "128" }},
		{"underflow", "outside int8_t", func(m *cabiCompatibilityManifest) { m.Functions[0].ResultType.Members[0].Value = "-129" }},
		{"unsigned negative", "outside uint8_t", func(m *cabiCompatibilityManifest) {
			m.Functions[0].Result = "uint8_t"
			m.Functions[0].ResultType.Members[0].Value = "-1"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := rewriteCABIManifestForTest(t, valid, test.mutate)
			for _, pair := range [][2][]byte{{data, valid}, {valid, data}} {
				if _, err := CompareCABIManifests(pair[0], pair[1]); err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("err=%v want=%s", err, test.want)
				}
			}
		})
	}
}
