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
- Native `struct` declarations are nominal Go-style values whose public fields retain their source names in JSON.
- `class` declarations are reference types with identity, constructors, visibility, optional inheritance, and JSON support for public state.

These forms are intentionally distinct. Choose them based on representation and ownership rather than appearance alone.

## Enums, aliases, and defined types

Enums are distinct integer types. `alias Name = T` is transparent. `type Name = distinct T` creates a nominal defined type with explicit conversions and its own method set.

```ts
enum Status: uint16 { Pending, Running = 4, Complete }
alias UserNames = string[];
type UserID = distinct int64;
```

## Generics and constraints

Functions, structs, interfaces, classes, defined types, and aliases can declare
type parameters. Use `extends comparable` when values must support equality or
serve as map keys:

```ts
function equal<T extends comparable>(left: T, right: T): boolean {
  return left === right;
}
```

Standard-library and installed Go-module constraints are available through
normal Go imports. The compiler checks both type arguments and which operators
the complete Go type set permits:

```ts
import go cmp from "cmp";

function minimum<T extends cmp.Ordered>(left: T, right: T): T {
  if (left < right) { return left; }
  return right;
}
```

`cmp.Ordered` accepts integers, floating-point values, strings, and defined
types with those underlying representations. An external integer-only
constraint also enables integer arithmetic, remainder, bitwise operators, and
shifts. Source-declared type-set expressions are not currently part of the
language; publish them from a Go package and import them instead.
