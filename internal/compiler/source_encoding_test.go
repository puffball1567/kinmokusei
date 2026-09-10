package compiler

import (
	"path/filepath"
	"testing"
)

func TestMalformedUTF8ReportedBeforeCodegen(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		"function value(): string { return \"\xd4\"; }",
		"/* \xff */ function value(): int { return 1; }",
		"// \xff\nfunction value(): int { return 1; }",
	} {
		path := filepath.Join(t.TempDir(), "encoding.km")
		result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: input})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != "invalid UTF-8 encoding" || result.Diagnostics[0].Span.Path != path {
			t.Fatalf("expected a source encoding diagnostic, got %v", result.Diagnostics)
		}
		if _, accepted, err := compilePipelineProperty(input); err != nil || accepted {
			t.Fatalf("malformed source reached codegen: accepted=%v err=%v", accepted, err)
		}
	}
}

func TestByteEscapesStillGenerateValidGo(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`function value(): string { return "\xFF\324"; }`,
		`function value(): string { return "日本語 😀 �"; }`,
	} {
		if _, accepted, err := compilePipelineProperty(input); err != nil || !accepted {
			t.Fatalf("valid string failed codegen: accepted=%v err=%v", accepted, err)
		}
	}
}
