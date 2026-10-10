package sema

import (
	"strings"
	"testing"
)

func TestNativeCollectionFieldAssignments(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{
		"let items: Item[] = []; items = append(items, Item { name: \"a\" });",
		"let items = makeSlice[Item](0); items = append(items, Item { name: \"a\" });",
		"let items = [Item { name: \"a\" }];",
	} {
		input := "struct Item { public name: string; } struct Group { public items: Item[]; } function make(): Group { " + declaration + " return Group { items: items }; }"
		if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
			t.Errorf("local slice rejected: %v", diagnostics)
		}
	}
	for name, input := range map[string]string{
		"class method": `struct Item { public name: string; } struct Group { public items: Item[]; } class Builder { function make(): Group { let items = [Item { name: "a" }]; return Group { items: items }; } }`,
		"generic":      `struct Item<T> { public value: T; } struct Group<T> { public items: Item<T>[]; } function make(): Group<int> { let items = [Item<int> { value: 1 }]; return Group<int> { items: items }; }`,
		"fixed array":  `struct Item { public name: string; } struct Group { public items: [1]Item; } function make(): Group { let items: [1]Item = [Item { name: "a" }]; return Group { items: items }; }`,
		"map":          `struct Item { public name: string; } struct Group { public items: Map<string, Item>; } function make(): Group { let items = makeMap[string, Item](); items["a"] = Item { name: "a" }; return Group { items: items }; }`,
		"nested":       `struct Item { public name: string; } struct Group { public items: Item[][]; } function make(): Group { let items = [[Item { name: "a" }]]; return Group { items: items }; }`,
		"forward":      `struct Group { public items: Item[]; } struct Item { public name: string; } function make(items: Item[]): Group { return Group { items: items }; }`,
	} {
		t.Run(name, func(t *testing.T) {
			if diagnostics := checkSource(t, input); len(diagnostics) != 0 {
				t.Fatalf("diagnostics: %v", diagnostics)
			}
		})
	}
}

func TestNativeCollectionFieldRejectsDifferentTypes(t *testing.T) {
	t.Parallel()
	for _, input := range []string{
		`struct Item { public name: string; } struct Other { public name: string; } struct Group { public items: Item[]; } function bad(items: Other[]): Group { return Group { items: items }; }`,
		`struct Item<T> { public value: T; } struct Group { public items: Item<int>[]; } function bad(items: Item<string>[]): Group { return Group { items: items }; }`,
		`struct Item { public name: string; } struct Group { public items: [2]Item; } function bad(items: [1]Item): Group { return Group { items: items }; }`,
		`struct Item { public name: string; } struct Group { public items: Map<int, Item>; } function bad(items: Map<string, Item>): Group { return Group { items: items }; }`,
		`struct Group { public items: string[]; } function bad(items: bstring[]): Group { return Group { items: items }; }`,
		`struct Item<T> { public value: T; } struct Group { public items: Item<string>[]; } function bad(items: Item<bstring>[]): Group { return Group { items: items }; }`,
		`class User {} struct Item<T> { public value: T; } struct Group { public items: Item<User>[]; } function bad(items: Item<User | null>[]): Group { return Group { items: items }; }`,
	} {
		diagnostics := checkSource(t, input)
		found := false
		for _, diagnostic := range diagnostics {
			if strings.Contains(diagnostic, "cannot use") {
				found = true
			}
		}
		if !found {
			t.Errorf("different types accepted: %v", diagnostics)
		}
	}
}
