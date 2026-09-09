package compiler

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestDifferentialAssertionContract(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name: "compares both implementations",
			source: `package sample
import ("testing"; reference "sample.test/reference")
func TestValue(t *testing.T) { want := reference.Value(); if got := value(); got != want { t.Fatalf("got %v want %v", got, want) } }`,
		},
		{
			name: "direct comparison",
			source: `package sample
import ("testing"; reference "sample.test/reference")
func TestValue(t *testing.T) { if value() != reference.Value() { t.Error("mismatch") } }`,
		},
		{
			name: "reference only imported",
			source: `package sample
import ("testing"; _ "sample.test/reference")
func TestValue(t *testing.T) { if value() != 1 { t.Error("mismatch") } }`,
			wantErr: "reference is imported but never evaluated",
		},
		{
			name: "results called but not compared",
			source: `package sample
import ("testing"; reference "sample.test/reference")
func TestValue(t *testing.T) { _ = value(); _ = reference.Value() }`,
			wantErr: "no failing test assertion",
		},
		{
			name: "failure depends only on generated result",
			source: `package sample
import ("testing"; reference "sample.test/reference")
func TestValue(t *testing.T) { _ = reference.Value(); if value() != 1 { t.Error("mismatch") } }`,
			wantErr: "no failing test assertion",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "sample_test.go", test.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			err = validateDifferentialAssertions(file, "sample.test")
			if test.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}
