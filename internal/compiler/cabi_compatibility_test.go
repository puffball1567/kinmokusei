package compiler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func emitCABIManifestForTest(t *testing.T, source string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "library.km")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, diagnostics, err := EmitCABI([]string{path})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	return artifacts.Manifest
}

func TestCompareCABIManifestsCompatibilityMatrix(t *testing.T) {
	base := emitCABIManifestForTest(t, `export c("kinmokusei_value") function value(input: int32): int32 { return input; }`)
	additive := emitCABIManifestForTest(t, `export c("kinmokusei_value") function value(input: int32): int32 { return input; } export c("kinmokusei_ping") function ping(): void {}`)
	changed := emitCABIManifestForTest(t, `export c("kinmokusei_value") function value(input: int64): int32 { return int32(input); }`)
	renamed := emitCABIManifestForTest(t, `export c("kinmokusei_other") function value(input: int32): int32 { return input; }`)

	tests := []struct {
		name         string
		baseline     []byte
		current      []byte
		exact        bool
		additions    []string
		breakingText []string
	}{
		{"exact", base, base, true, nil, nil},
		{"additive", base, additive, false, []string{"kinmokusei_ping"}, nil},
		{"removal", additive, base, false, nil, []string{`removed C ABI symbol "kinmokusei_ping"`}},
		{"signature", base, changed, false, nil, []string{`changed signature of C ABI symbol "kinmokusei_value"`}},
		{"rename", base, renamed, false, []string{"kinmokusei_other"}, []string{`removed C ABI symbol "kinmokusei_value"`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := CompareCABIManifests(test.baseline, test.current)
			if err != nil {
				t.Fatal(err)
			}
			if result.ExactFingerprint != test.exact || !slicesEqual(result.Additions, test.additions) || result.Compatible() != (len(test.breakingText) == 0) {
				t.Fatalf("result=%#v", result)
			}
			joined := strings.Join(result.BreakingChanges, "\n")
			for _, want := range test.breakingText {
				if !strings.Contains(joined, want) {
					t.Fatalf("breaking=%v want=%q", result.BreakingChanges, want)
				}
			}
		})
	}
}

func TestCompareCABIManifestsGatewayAndStatusMatrix(t *testing.T) {
	base := emitCABIManifestForTest(t, `export c("kinmokusei_value") function value(): void {}`)
	majorChanged := rewriteCABIManifestForTest(t, base, func(manifest *cabiCompatibilityManifest) {
		manifest.GatewayVersion.Major++
	})
	minorBaseline := rewriteCABIManifestForTest(t, base, func(manifest *cabiCompatibilityManifest) {
		manifest.GatewayVersion.Minor = 1
	})
	minorCurrent := rewriteCABIManifestForTest(t, base, func(manifest *cabiCompatibilityManifest) {
		manifest.GatewayVersion.Minor = 1
	})
	statusChanged := rewriteCABIManifestForTest(t, base, func(manifest *cabiCompatibilityManifest) {
		manifest.Status.Panic = 9
	})
	tests := []struct {
		name     string
		baseline []byte
		current  []byte
		want     string
	}{
		{"major", base, majorChanged, "gateway major version changed"},
		{"minor downgrade", minorBaseline, base, "gateway minor version decreased"},
		{"status", base, statusChanged, "status code contract changed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := CompareCABIManifests(test.baseline, test.current)
			if err != nil || result.Compatible() || !strings.Contains(strings.Join(result.BreakingChanges, "\n"), test.want) {
				t.Fatalf("result=%#v err=%v want=%q", result, err, test.want)
			}
		})
	}
	if result, err := CompareCABIManifests(base, minorCurrent); err != nil || !result.Compatible() || result.ExactFingerprint {
		t.Fatalf("minor upgrade result=%#v err=%v", result, err)
	}
}

func TestCompareCABIManifestsRejectsInvalidMatrix(t *testing.T) {
	valid := emitCABIManifestForTest(t, `export c("kinmokusei_value") function value(): void {}`)
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "invalid baseline"},
		{"malformed", []byte(`{"schemaVersion":`), "invalid baseline"},
		{"unknown field", bytes.Replace(valid, []byte(`"schemaVersion": 1`), []byte(`"schemaVersion": 1, "unknown": true`), 1), "unknown field"},
		{"unsupported schema", bytes.Replace(valid, []byte(`"schemaVersion": 1`), []byte(`"schemaVersion": 2`), 1), "schema version 2"},
		{"tampered fingerprint", bytes.Replace(valid, []byte("sha256:"), []byte("sha256:x"), 1), "manifest fingerprint"},
		{"trailing JSON", append(append([]byte(nil), valid...), []byte(` {}`)...), "multiple JSON values"},
		{"duplicate symbol", rewriteCABIManifestForTest(t, valid, func(manifest *cabiCompatibilityManifest) {
			manifest.Functions = append(manifest.Functions, manifest.Functions[0])
		}), "unique symbols"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := CompareCABIManifests(test.data, valid); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want=%q", err, test.want)
			}
		})
	}
}

func rewriteCABIManifestForTest(t *testing.T, data []byte, mutate func(*cabiCompatibilityManifest)) []byte {
	t.Helper()
	var manifest cabiCompatibilityManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	mutate(&manifest)
	canonical, err := json.Marshal(cabiCompatibilityFingerprintInput{
		SchemaVersion: manifest.SchemaVersion, GatewayVersion: manifest.GatewayVersion, Status: manifest.Status, Functions: manifest.Functions,
	})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	manifest.Fingerprint = "sha256:" + hex.EncodeToString(digest[:])
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
