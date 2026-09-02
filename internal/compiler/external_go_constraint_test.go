package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalGoIntegerConstraintMatchesIndependentGo(t *testing.T) {
	root := t.TempDir()
	library := filepath.Join(root, "constraints")
	if err := os.MkdirAll(library, 0o755); err != nil {
		t.Fatal(err)
	}
	mainModule := `module example.com/application

go 1.23

require example.com/constraints v0.0.0

replace example.com/constraints => ./constraints
`
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(mainModule), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(library, "go.mod"), []byte("module example.com/constraints\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	librarySource := `package constraints
type Integer interface {
  ~int | ~int8 | ~int16 | ~int32 | ~int64 |
  ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}
`
	if err := os.WriteFile(filepath.Join(library, "constraints.go"), []byte(librarySource), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "main.otm")
	ontamaSource := `
import go constraints from "example.com/constraints";

type Score = distinct int;
type Counters<T extends constraints.Integer> = distinct T[];

function transform<T extends constraints.Integer>(left: T, right: T, shift: uint): T {
  let value = (left + right) ^ left;
  value |= right;
  return value << shift;
}
function scoreTransform(left: int, right: int, shift: uint): int {
  return int(transform(Score(left), Score(right), shift));
}
function byteTransform(left: byte, right: byte, shift: uint): byte {
  return transform(left, right, shift);
}
function counterLength(values: int[]): int { return len(Counters<int>(values)); }
`
	if err := os.WriteFile(source, []byte(ontamaSource), 0o644); err != nil {
		t.Fatal(err)
	}
	directory, diagnostics, err := WriteGeneratedModule([]string{source}, "customconstraint")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("directory=%q err=%v diagnostics=%v", directory, err, diagnostics)
	}
	generated, err := os.ReadFile(filepath.Join(directory, "generated.go"))
	if err != nil || !strings.Contains(string(generated), `T constraints.Integer`) {
		t.Fatalf("generated constraint missing: err=%v\n%s", err, generated)
	}
	referenceSource := `package reference
import constraints "example.com/constraints"
type Score int
type Counters[T constraints.Integer] []T
func transform[T constraints.Integer](left, right T, shift uint) T {
  value := (left + right) ^ left
  value |= right
  return value << shift
}
func ScoreTransform(left, right int, shift uint) int { return int(transform(Score(left), Score(right), shift)) }
func ByteTransform(left, right byte, shift uint) byte { return transform(left, right, shift) }
func CounterLength(values []int) int { return len(Counters[int](values)) }
`
	testSource := `package customconstraint
import (
  "testing"
  reference "customconstraint.test/reference"
)
func TestBehavior(t *testing.T) {
  for _, item := range []struct{ left, right int; shift uint }{{0, 0, 0}, {-7, 3, 0}, {5, 9, 1}, {100, 2, 3}} {
    if got, want := scoreTransform(item.left, item.right, item.shift), reference.ScoreTransform(item.left, item.right, item.shift); got != want { t.Errorf("scoreTransform(%v) = %d, Go = %d", item, got, want) }
  }
  for _, item := range []struct{ left, right byte; shift uint }{{0, 0, 0}, {1, 2, 1}, {200, 7, 2}, {255, 1, 3}} {
    if got, want := byteTransform(item.left, item.right, item.shift), reference.ByteTransform(item.left, item.right, item.shift); got != want { t.Errorf("byteTransform(%v) = %d, Go = %d", item, got, want) }
  }
  for _, values := range [][]int{nil, {}, {0}, {-1, 2, 3}} {
    if got, want := counterLength(values), reference.CounterLength(values); got != want { t.Errorf("counterLength(%v) = %d, Go = %d", values, got, want) }
  }
}
`
	differentialModule := `module customconstraint.test

go 1.23

require example.com/constraints v0.0.0

replace example.com/constraints => ../../constraints
`
	t.Setenv("GOPROXY", "off")
	runGeneratedGoDifferentialTestWithModule(t, directory, "customconstraint.test", differentialModule, generated, referenceSource, testSource)
}
