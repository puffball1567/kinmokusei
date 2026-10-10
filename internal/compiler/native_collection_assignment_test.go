package compiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeCollectionFieldsMatchIndependentGo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "collections.km")
	input := `struct Item<T> { public value: T; }
struct Group { public items: Item<int>[]; }
struct Fixed { public items: [1]Item<int>; }
struct Indexed { public items: Map<int, Item<int>>; }
function Typed(): int { let items: Item<int>[] = []; items = append(items, Item<int> { value: 1 }); let group = Group { items: items }; group.items[0].value = 9; return items[0].value; }
function Made(): int { let items = makeSlice[Item<int>](0); items = append(items, Item<int> { value: 2 }); return Group { items: items }.items[0].value; }
function Inferred(): int { let items = [Item<int> { value: 3 }]; return Group { items: items }.items[0].value; }
function Copied(): int { let items: [1]Item<int> = [Item<int> { value: 4 }]; let group = Fixed { items: items }; group.items[0].value = 9; return items[0].value * 10 + group.items[0].value; }
function Mapped(): int { let items = makeMap[int, Item<int>](); items[0] = Item<int> { value: 5 }; let group = Indexed { items: items }; group.items[0] = Item<int> { value: 9 }; return items[0].value; }
class Builder { public function make(): Group { let items = [Item<int> { value: 6 }]; return Group { items: items }; } }
function Method(): int { return new Builder().make().items[0].value; }
`
	if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, diagnostics, err := EmitGo([]string{path}, "collections")
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("emit: %v %v", err, diagnostics)
	}
	reference := `package reference
type Item[T any] struct { Value T }
type Group struct { Items []Item[int] }
type Fixed struct { Items [1]Item[int] }
type Indexed struct { Items map[int]Item[int] }
func Typed() int { items := []Item[int]{}; items = append(items, Item[int]{Value:1}); group := Group{Items:items}; group.Items[0].Value = 9; return items[0].Value }
func Made() int { items := make([]Item[int], 0); items = append(items, Item[int]{Value:2}); return Group{Items:items}.Items[0].Value }
func Inferred() int { items := []Item[int]{{Value:3}}; return Group{Items:items}.Items[0].Value }
func Copied() int { items := [1]Item[int]{{Value:4}}; group := Fixed{Items:items}; group.Items[0].Value=9; return items[0].Value*10+group.Items[0].Value }
func Mapped() int { items:=make(map[int]Item[int]); items[0]=Item[int]{Value:5}; group:=Indexed{Items:items}; group.Items[0]=Item[int]{Value:9}; return items[0].Value }
type Builder struct{}
func (builder *Builder) Make() Group { items:=[]Item[int]{{Value:6}}; return Group{Items:items} }
func Method() int { return (&Builder{}).Make().Items[0].Value }
`
	tests := `package collections
import ("testing"; "nativecollections/reference")
func TestCollections(t *testing.T) {
 if got,want:=Typed(),reference.Typed();got!=want{t.Fatalf("typed=%d,want=%d",got,want)}
 if got,want:=Made(),reference.Made();got!=want{t.Fatalf("made=%d,want=%d",got,want)}
 if got,want:=Inferred(),reference.Inferred();got!=want{t.Fatalf("inferred=%d,want=%d",got,want)}
 if got,want:=Copied(),reference.Copied();got!=want{t.Fatalf("copied=%d,want=%d",got,want)}
 if got,want:=Mapped(),reference.Mapped();got!=want{t.Fatalf("mapped=%d,want=%d",got,want)}
 if got,want:=Method(),reference.Method();got!=want{t.Fatalf("method=%d,want=%d",got,want)}
}
`
	runGeneratedGoDifferentialTest(t, root, "nativecollections", generated, reference, tests)
}
