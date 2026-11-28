package compiler

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoExportBoundOrder(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		declaration string
		want        string
	}{
		{"public function", `export function Equal<T extends Pair<E>, E extends comparable>(a:T,b:T):boolean{return a===b;}`, "Equal"},
		{"public struct", `export struct Box<T extends Pair<E>, E extends comparable>{public value:T;}`, "Box"},
		{"public interface", `export interface Reader<T extends Pair<E>, E extends comparable>{function read():T;}`, "Reader"},
		{"public defined type", `export type Values<T extends Pair<E>, E extends comparable>=distinct T[];`, "Values"},
		{"public constructor", `class box<T extends Pair<E>, E extends comparable>{}`, "box"},
		{"private function", `function equal<T extends Pair<E>, E extends comparable>(a:T,b:T):boolean{return a===b;}`, ""},
		{"reachable private function", `function equal<T extends Pair<E>, E extends comparable>(a:T,b:T):boolean{return a===b;}
function compare(a:[2]int,b:[2]int):boolean{return equal(a,b);}
export function Equal(a:[2]int,b:[2]int):boolean{return compare(a,b);}`, "equal"},
		{"reordered function", `export function Equal<E extends comparable, T extends Pair<E>>(a:T,b:T):boolean{return a===b;}`, ""},
		{"forward non-filtering bound", `export function Keep<T extends Array<E>, E extends comparable>(a:T):T{return a;}`, ""},
		{"forward pointer bound", `export function Keep<T extends Pointer<E>, E extends comparable>(a:T):T{return a;}`, ""},
		{"erased alias", `export alias Values<T extends Pair<E>, E extends comparable>=T[];`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.km")
			const declarations = `constraint RawArray<E>=~[2]E;
constraint Pair<E>=RawArray<E>&comparable;
constraint Pointer<E>=~*E&comparable;
constraint Array<E>=RawArray<E>;
`
			result, err := CheckFilesWithOverlay([]string{path}, map[string]string{path: declarations + test.declaration})
			if err != nil {
				t.Fatal(err)
			}
			if test.want == "" {
				if len(result.Diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
				}
				return
			}
			want := "Go export data for \"" + test.want + "\" would evaluate a comparable constraint"
			if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, want) ||
				result.Diagnostics[0].Span.Start.Line != 5 {
				t.Fatalf("want a source diagnostic containing %q on line 5, got %v", want, result.Diagnostics)
			}
		})
	}
}
