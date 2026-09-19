package sema

import (
	gotypes "go/types"
	"strings"
	"testing"
)

func TestParameterConstraintDisjointIntersections(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input, want string }{
		{"slice array", `constraint A<E>=~E[]|~[2]E;constraint B<E>=~E[]|~[3]E;constraint C<E>=A<E>&B<E>;function first<E,T extends C<E>>(xs:T):E{return xs[0];}function use(xs:int[]):int{return first(xs);}`, ""},
		{"channel directions", `constraint S<E>=~GoChannel<E>|~GoSendChannel<E>;constraint R<E>=~GoChannel<E>|~GoReceiveChannel<E>;constraint C<E>=S<E>&R<E>;function relay<E,T extends C<E>>(ch:T,x:E):E{ch<-x;return <-ch;}`, ""},
		{"named exact", `type A<E>=distinct E[];type B<E>=distinct E[];constraint X<E>=A<E>|B<E>;constraint Y<E>=A<E>;constraint Z<E>=X<E>&Y<E>;function first<E,T extends Z<E>>(xs:T):E{return xs[0];}`, ""},
		{"named argument shapes", `type A<E>=distinct E[];constraint X<E>=A<E[]>|A<[2]E>;constraint Z<E>=X<E>&A<E[]>;`, ""},
		{"named underlying overlap", `type A<E>=distinct E[];constraint X<E>=A<E>|~[2]E;constraint Z<E>=X<E>&~E[];function first<E,T extends Z<E>>(xs:T):E{return xs[0];}`, ""},
		{"map slice", `constraint A<E>=~Map<string,E>|~E[];constraint C<E>=A<E>&~Map<string,E>;function get<E,T extends C<E>>(m:T):E{return m["key"];}`, ""},
		{"nested pointer", `constraint A<E>=~*E[]|~*[2]E;constraint C<E>=A<E>&~*E[];`, ""},
		{"different array lengths", `constraint A<E>=~[2]E|~[3]E;constraint C<E>=A<E>&~[2]E;`, ""},
		{"nested channel directions", `constraint A<E>=~GoSendChannel<E>[]|~GoReceiveChannel<E>[];constraint C<E>=A<E>&~GoSendChannel<E>[];`, ""},
		{"nested fixed scalar", `constraint A<E>=~Map<int,E>[]|~Map<string,E>[];constraint C<E>=A<E>&~Map<int,E>[];`, ""},
		{"empty shapes", `constraint C<E>=~E[]&~[2]E;`, "no common types"},
		{"empty direction", `constraint C<E>=~GoSendChannel<E>&~GoReceiveChannel<E>;`, "no common types"},
		{"excluded array", `constraint A<E>=~E[]|~[2]E;constraint C<E>=A<E>&~E[];function f<E,T extends C<E>>(xs:T):void{}function bad(xs:[2]int):void{f<int,[2]int>(xs);}`, "does not satisfy"},
		{"dependent elements", `constraint C<E>=~E[]&~int[];`, "unmatched parameter-dependent"},
		{"dependent map keys", `constraint C<K extends comparable,E>=~Map<K,E>&~Map<int,E>;`, "unmatched parameter-dependent"},
		{"dependent nested", `constraint C<E,F>=~*E[]&~*F[];`, "unmatched parameter-dependent"},
		{"named dependent arguments", `type A<E>=distinct E[];constraint C<E>=A<E>&A<int>;`, "unmatched parameter-dependent"},
		{"nullable shape", `class Item{}alias Maybe=Item|null;constraint A<E>=~E[]|~[2]E;constraint B<E>=A<E>&~E[];function get<E,T extends B<E>>(xs:T):E{return xs[0];}function bad(xs:Maybe[]):Item{return get(xs);}`, "cannot use"},
		{"nullable conflict", `class Item{}alias Maybe=Item|null;constraint A<E>=~E[]|~[2]E;constraint C=A<Item>&~Maybe[];`, "incompatible nullable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := strings.Join(checkSource(t, test.input), "\n")
			if test.want == "" && got != "" || test.want != "" && !strings.Contains(got, test.want) {
				t.Fatalf("diagnostics=%s want=%q", got, test.want)
			}
		})
	}
}

// Every proven exclusion must still be disjoint after independently replacing
// both parameters. This guards against confusing assignment with type identity.
func TestConstraintDisjointnessSurvivesSubstitution(t *testing.T) {
	t.Parallel()
	any := gotypes.NewInterfaceType(nil, nil).Complete()
	e := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "E", nil), any)
	f := gotypes.NewTypeParam(gotypes.NewTypeName(0, nil, "F", nil), any)
	i, s := gotypes.Typ[gotypes.Int], gotypes.Typ[gotypes.String]
	named := func(name string) *gotypes.Named {
		return gotypes.NewNamed(gotypes.NewTypeName(0, nil, name, nil), gotypes.NewSlice(i), nil)
	}
	a, b := named("A"), named("B")
	patterns := []gotypes.Type{e, f, i, s, a, b}
	generic := gotypes.NewNamed(gotypes.NewTypeName(0, nil, "Box", nil), gotypes.NewSlice(e), nil)
	generic.SetTypeParams([]*gotypes.TypeParam{e})
	for _, argument := range []gotypes.Type{e, f, gotypes.NewSlice(e), gotypes.NewArray(e, 2)} {
		instance, err := gotypes.Instantiate(nil, generic, []gotypes.Type{argument}, false)
		if err != nil {
			t.Fatal(err)
		}
		patterns = append(patterns, instance)
	}
	for _, value := range []gotypes.Type{e, f, i} {
		patterns = append(patterns, gotypes.NewSlice(value), gotypes.NewArray(value, 2), gotypes.NewArray(value, 3),
			gotypes.NewPointer(value), gotypes.NewMap(s, value), gotypes.NewChan(gotypes.SendRecv, value),
			gotypes.NewChan(gotypes.SendOnly, value), gotypes.NewChan(gotypes.RecvOnly, value),
			gotypes.NewSlice(gotypes.NewMap(i, value)), gotypes.NewSlice(gotypes.NewMap(s, value)))
	}
	arguments := []gotypes.Type{i, s, a, gotypes.NewSlice(i), gotypes.NewArray(i, 2), gotypes.NewChan(gotypes.SendOnly, i)}
	for _, left := range patterns {
		for _, right := range patterns {
			for _, tilde := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
				a, b := gotypes.NewTerm(tilde[0], left), gotypes.NewTerm(tilde[1], right)
				if !constraintTermsProvablyDisjoint(a, b) {
					continue
				}
				for _, x := range arguments {
					for _, y := range arguments {
						bindings := map[gotypes.Type]gotypes.Type{e: x, f: y}
						l := gotypes.NewTerm(a.Tilde(), substituteConstraintType(a.Type(), bindings))
						r := gotypes.NewTerm(b.Tilde(), substituteConstraintType(b.Type(), bindings))
						if typeSetTermsOverlap(l, r) {
							t.Fatalf("%s and %s overlap after E=%s F=%s", a, b, x, y)
						}
					}
				}
			}
		}
	}
}
