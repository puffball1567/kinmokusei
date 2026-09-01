# Types and data

OnsenTamago preserves Go-compatible representations where they matter. It does not collapse integers into `number`, erase named types, or hide pointer and collection behavior.

## Scalar types

- `boolean`, `string`, and `void`
- `int`, `uint`, and fixed-width signed/unsigned integers
- `byte` and the Go-compatible `uint8` alias
- `float32`, `float64`, `float`, and `number`

`number`, `float`, and `float64` are the same type. `int` remains a separate integer type.

## Collections

```ts
const values: int[] = [1, 2, 3];
const pair: [2]string = ["hot", "spring"];
const scores = makeMap[string, int]();
scores["onsen"] = 42;
```

Slices alias backing storage as Go slices do. Fixed arrays copy on assignment and parameter passing. Maps are reference-bearing values and have unspecified iteration order.

## Objects, structs, and classes

- Structural object types are convenient data-transfer values and generate anonymous Go structs with JSON tags.
- Native `struct` declarations are nominal Go-style values.
- `class` declarations are reference types with identity, constructors, visibility, and optional inheritance.

These forms are intentionally distinct. Choose them based on representation and ownership rather than appearance alone.

## Enums, aliases, and defined types

Enums are distinct integer types. `alias Name = T` is transparent. `type Name = distinct T` creates a nominal defined type with explicit conversions and its own method set.

```ts
enum Status: uint16 { Pending, Running = 4, Complete }
alias UserNames = string[];
type UserID = distinct int64;
```
