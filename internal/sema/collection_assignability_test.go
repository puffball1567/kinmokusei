package sema

import (
	"strings"
	"testing"
)

func TestConstrainedClassCollectionAssignment(t *testing.T) {
	const declarations = `class Item{public value:int=1;}class Other{}class Child extends Item{}alias Maybe=Item|null;`
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"slice", `constraint S=~Item[];function f<T extends S>(v:T):Item[]{return v;}`, true},
		{"map", `constraint S=~Map<string,Item>;function f<T extends S>(v:T):Map<string,Item>{return v;}`, true},
		{"array", `constraint S=~[2]Item;function f<T extends S>(v:T):[2]Item{return v;}`, true},
		{"channel", `constraint S=~GoChannel<Item>|~GoReceiveChannel<Item>;function f<T extends S>(v:T):GoReceiveChannel<Item>{return v;}`, true},
		{"nullable", `constraint S=~Maybe[];function f<T extends S>(v:T):Maybe[]{return v;}`, true},
		{"narrow nullable", `constraint S=~Maybe[];function f<T extends S>(v:T):Item[]{return v;}`, false},
		{"widen writable nullable", `constraint S=~Item[];function f<T extends S>(v:T):Maybe[]{return v;}`, false},
		{"invariant inheritance", `constraint S=~Child[];function f<T extends S>(v:T):Item[]{return v;}`, false},
		{"nominal elements", `constraint S=~Other[];function f<T extends S>(v:T):Item[]{return v;}`, false},
		{"array length", `constraint S=~[3]Item;function f<T extends S>(v:T):[2]Item{return v;}`, false},
		{"channel direction", `constraint S=~GoReceiveChannel<Item>;function f<T extends S>(v:T):GoChannel<Item>{return v;}`, false},
		{"receive class", `function f(v:GoReceiveChannel<Item>):Item{return <-v;}`, true},
		{"checked receive class", `function f(v:GoReceiveChannel<Item>):Item{const [value,ok]=<-v;return value;}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, declarations+tc.body), "\n")
			if tc.valid && got != "" || !tc.valid && !strings.Contains(got, "cannot use") {
				t.Fatalf("valid=%v diagnostics=%s", tc.valid, got)
			}
		})
	}
}
