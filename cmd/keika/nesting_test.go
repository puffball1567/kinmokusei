package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDeepSyntaxDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep.km")
	if err := os.WriteFile(path, []byte("const x="+strings.Repeat("!", 80000)+"true;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, stdout, stderr := captureRun(t, "check", path); status != 1 || stdout != "" || !strings.Contains(stderr, "syntax nesting is too deep") || strings.Contains(stderr, "fatal error") {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	status, stdout, stderr := captureRun(t, "check", "--json", path)
	if status != 1 || stderr != "" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout, stderr)
	}
	var report struct {
		Valid       bool `json:"valid"`
		Diagnostics []struct {
			Message string                     `json:"message"`
			Path    string                     `json:"path"`
			Start   struct{ Line, Column int } `json:"start"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if report.Valid || len(report.Diagnostics) != 1 {
		t.Fatalf("report=%#v", report)
	}
	got := report.Diagnostics[0]
	if got.Path != path || got.Start.Line != 1 || got.Start.Column < 1 || !strings.Contains(got.Message, "syntax nesting is too deep") {
		t.Fatalf("diagnostic=%#v", got)
	}
}
