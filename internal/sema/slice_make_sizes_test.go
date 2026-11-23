package sema

import (
	"strings"
	"testing"
)

func TestSliceMakeSizeResults(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"named sizes", `type Size=distinct uint;function sizes():(Size,int16){return Size(1),3;}function f():int[]{return makeSlice<int>(sizes());}`, ""},
		{"constrained sizes", `constraint Int=~int|~uint64;function sizes<N extends Int>(n:N):(N,N){return n,n;}function f<N extends Int>(n:N):int[]{return make<int[]>(sizes(n));}`, ""},
		{"nullable elements", `class Item{}alias Slot=Item|null;function sizes():(int,int){return 1,3;}function f():Slot[]{return makeSlice<Slot>(sizes());}`, ""},
		{"first result", `function sizes():(float64,int){return 1,3;}function f():int[]{return makeSlice<int>(sizes());}`, "size result 1 must be an integer"},
		{"second result", `function sizes():(int,string){return 1,"3";}function f():int[]{return make<int[]>(sizes());}`, "size result 2 must be an integer"},
		{"count", `function sizes():(int,int,int){return 1,3,5;}function f():int[]{return makeSlice<int>(sizes());}`, "expects two results for length and capacity"},
		{"mixed with other argument", `function sizes():(int,int){return 1,3;}function f():int[]{return makeSlice<int>(sizes(),4);}`, "multiple values"},
		{"map", `function sizes():(int,int){return 1,3;}function f():Map<int,int>{return make<Map<int,int>>(sizes());}`, "multiple values"},
		{"channel", `function sizes():(int,int){return 1,3;}function f():GoChannel<int>{return make<GoChannel<int>>(sizes());}`, "multiple values"},
		{"unsafe zero", `class Item{}function sizes():(int,int){return 0,3;}function f():Item[]{return makeSlice<Item>(sizes());}`, "cannot initialize elements"},
		{"Result effect", `function sizes():Result<int>{return ok(3);}function f():int[]{return make<int[]>(sizes());}`, "Result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics = %q, want %q", got, test.want)
			}
		})
	}
}
