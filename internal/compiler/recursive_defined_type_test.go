package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecursiveDefinedTypesMatchIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	source := filepath.Join(temporary, "recursive_defined.km")
	if err := os.WriteFile(source, []byte(`
type Chain = distinct Chain[];
type Tree = distinct Map<string, Tree>;
type Link = distinct *Link;
type Visitor = distinct (next: Visitor) => void;
type Stream = distinct GoChannel<Stream>;
type Left = distinct Right;
type Right = distinct *Left;
type Forest<T> = distinct Forest<T>[];

public function size(this: Chain): int { return len(this); }

function ChainBehavior(value: Chain): int {
  return value.size() * 100 + value[0].size();
}
function TreeBehavior(value: Tree): int {
  return len(value) * 100 + len(value["child"]);
}
function LinkBehavior(value: Link): boolean {
  if (value === nil) { return true; }
  return *value === value;
}
function Visit(visitor: Visitor): void { visitor(visitor); }
function StreamBehavior(value: Stream): boolean {
  value <- value;
  const received = <-value;
  return received === value;
}
function ForestBehavior<T>(value: Forest<T>): int {
  return len(value) * 100 + len(value[0]);
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{source}, "recursivedefined")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	for _, expected := range []string{
		"type Chain []Chain",
		"type Tree map[string]Tree",
		"type Link *Link",
		"type Visitor func(Visitor)",
		"type Stream chan Stream",
		"type Left Right",
		"type Right *Left",
		"type Forest[T any] []Forest[T]",
		"func (this Chain) Size() int",
	} {
		if !strings.Contains(string(generated), expected) {
			t.Errorf("generated Go does not contain %q:\n%s", expected, generated)
		}
	}

	referenceSource := `package reference

type Chain []Chain
type Tree map[string]Tree
type Link *Link
type Visitor func(next Visitor)
type Stream chan Stream
type Left Right
type Right *Left
type Forest[T any] []Forest[T]

func (value Chain) Size() int { return len(value) }
func ChainBehavior(value Chain) int { return value.Size()*100 + value[0].Size() }
func TreeBehavior(value Tree) int { return len(value)*100 + len(value["child"]) }
func LinkBehavior(value Link) bool { if value == nil { return true }; return *value == value }
func Visit(visitor Visitor) { visitor(visitor) }
func StreamBehavior(value Stream) bool { value <- value; received := <-value; return received == value }
func ForestBehavior[T any](value Forest[T]) int { return len(value)*100 + len(value[0]) }
`
	testSource := `package recursivedefined_test

import (
  "testing"
  generated "recursivedefined.test"
  reference "recursivedefined.test/reference"
)

func TestRecursiveDefinedBehavior(t *testing.T) {
  gotChain := generated.Chain{generated.Chain{}, generated.Chain{generated.Chain{}}}
  wantChain := reference.Chain{reference.Chain{}, reference.Chain{reference.Chain{}}}
  if got, want := generated.ChainBehavior(gotChain), reference.ChainBehavior(wantChain); got != want { t.Errorf("ChainBehavior = %d, Go = %d", got, want) }
  if got, want := gotChain.Size(), wantChain.Size(); got != want { t.Errorf("public Chain.Size = %d, Go = %d", got, want) }

  gotTree := generated.Tree{"child": generated.Tree{"leaf": generated.Tree{}}}
  wantTree := reference.Tree{"child": reference.Tree{"leaf": reference.Tree{}}}
  if got, want := generated.TreeBehavior(gotTree), reference.TreeBehavior(wantTree); got != want { t.Errorf("TreeBehavior = %d, Go = %d", got, want) }

  var gotLink generated.Link
  var wantLink reference.Link
  if got, want := generated.LinkBehavior(gotLink), reference.LinkBehavior(wantLink); got != want { t.Errorf("nil LinkBehavior = %v, Go = %v", got, want) }
  gotLink = &gotLink
  wantLink = &wantLink
  if got, want := generated.LinkBehavior(gotLink), reference.LinkBehavior(wantLink); got != want { t.Errorf("recursive LinkBehavior = %v, Go = %v", got, want) }

  gotCalls, wantCalls := 0, 0
  generated.Visit(generated.Visitor(func(next generated.Visitor) { gotCalls++; if next == nil { t.Error("generated visitor received nil") } }))
  reference.Visit(reference.Visitor(func(next reference.Visitor) { wantCalls++; if next == nil { t.Error("reference visitor received nil") } }))
  if gotCalls != wantCalls { t.Errorf("Visit calls = %d, Go = %d", gotCalls, wantCalls) }

  gotStream, wantStream := make(generated.Stream, 1), make(reference.Stream, 1)
  if got, want := generated.StreamBehavior(gotStream), reference.StreamBehavior(wantStream); got != want { t.Errorf("StreamBehavior = %v, Go = %v", got, want) }

  gotForest := generated.Forest[int]{generated.Forest[int]{}, generated.Forest[int]{generated.Forest[int]{}}}
  wantForest := reference.Forest[int]{reference.Forest[int]{}, reference.Forest[int]{reference.Forest[int]{}}}
  if got, want := generated.ForestBehavior(gotForest), reference.ForestBehavior(wantForest); got != want { t.Errorf("ForestBehavior = %d, Go = %d", got, want) }

  var gotLeft generated.Left
  var wantLeft reference.Left
  if (gotLeft == nil) != (wantLeft == nil) { t.Errorf("mutual recursive zero values differ") }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "recursivedefined.test", generated, referenceSource, testSource)
}

func TestLinkedRecursiveDefinedTypeMatchesIndependentGo(t *testing.T) {
	temporary := t.TempDir()
	dependency := filepath.Join(temporary, "chain.km")
	entry := filepath.Join(temporary, "entry.km")
	if err := os.WriteFile(dependency, []byte(`
type Chain = distinct Chain[];
public function size(this: Chain): int { return len(this); }
function score(value: Chain): int { return value.size() * 100 + value[0].size(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte(`
import { Chain, score } from "./chain";
function linked(value: Chain): int { return score(value) + value.size(); }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{entry}, "recursivedefinedlinked")
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%v", err, diagnostics)
	}
	referenceSource := `package reference
type Chain []Chain
func (value Chain) Size() int { return len(value) }
func score(value Chain) int { return value.Size()*100 + value[0].Size() }
func Linked(value Chain) int { return score(value) + value.Size() }
`
	testSource := `package recursivedefinedlinked
import (
  "testing"
  reference "recursivedefined-linked.test/reference"
)
func TestLinked(t *testing.T) {
  got := Chain{Chain{}, Chain{Chain{}}}
  want := reference.Chain{reference.Chain{}, reference.Chain{reference.Chain{}}}
  if gotValue, wantValue := linked(got), reference.Linked(want); gotValue != wantValue { t.Errorf("linked = %d, Go = %d", gotValue, wantValue) }
}
`
	runGeneratedGoDifferentialTest(t, temporary, "recursivedefined-linked.test", generated, referenceSource, testSource)
}
