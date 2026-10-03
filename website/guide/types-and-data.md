---
title: Types and data
description: Use Kinmokusei scalars, collections, nominal types, nullability, pointers, and conversions.
---

# Types and data

Kinmokusei preserves Go-compatible representations where they matter. It does not collapse integers into `number`, erase named types, or hide pointer and collection behavior.

## Scalar types

- `boolean`, `string`, `bstring`, and `void`
- `int`, `uint`, and fixed-width signed/unsigned integers
- `byte` and the Go-compatible `uint8` alias
- `float32`, `float64`, `float`, and `number`
- `complex64` and `complex128`, with imaginary literals and `complex` / `real` / `imag`

`number`, `float`, and `float64` are the same type. `int` remains a separate integer type.

## Text and bytes

This text-contract split is available in v0.4.6; older compilers have the
previous Go byte-string behavior. See [migration notes](../project/releases#migrating-from-v045).

`string` guarantees valid UTF-8 text in checked Kinmokusei code. `bstring`
stores arbitrary immutable bytes and represents strings received from Go.
`byte[]` is mutable byte storage. Both string types lower to Go `string`, not
JavaScript UTF-16. Length and indexing count bytes; `for-of` decodes `int32`
code points. A `string` slice checks UTF-8 boundaries and panics if it splits
a code point; a `bstring` slice permits any in-range byte boundary.

To edit encoded data, use a mutable byte slice via `alias Bytes = byte[]` and
`Bytes(text)`, then validate it back with `const text = string(bytes)?` inside
a `Result` function. Alternatively split the result and handle its error.
Invalid input is rejected, not silently replaced. Use `bstring(bytes)` when
raw bytes are intended, or `b"\xff"` for a raw literal. For code-point editing,
use an `int32[]` alias instead. Those conversions preserve independence from
later slice mutations. Use `strconv`/`fmt` for numeric formatting, not
`string(number)`, which encodes one code point. Follow the
[string recipe](../examples/unicode-strings#conversion-and-validation) and
[detailed string rules](../book/types-and-values#strings-and-unicode).

## Collections

```ts
const values: int[] = [1, 2, 3];
const pair: [2]string = ["hot", "spring"];
const scores = makeMap[string, int]();
scores["hello"] = 42;
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

`cmp.Ordered` accepts integers, floating-point values, raw `bstring` and defined
types with those underlying representations. Source constraints can describe
type sets directly:

```ts
constraint Whole = ~int | ~int64

function addWhole<T extends Whole>(left: T, right: T): T {
  return left + right
}
```

Exact terms name one type; `~T` includes types with that underlying shape.
Unions use `|`, intersections use `&`, and compatible Go method interfaces can
add behavior requirements. These are compile-time bounds, not runtime values.
Use `~string` in a native constraint for verified text; imported Go string
terms have the raw contract. Unbounded parameters must not erase unknown text
storage at an opaque Go boundary; use a provable raw/numeric bound there.
Every operation in generic code must be valid for the whole set. See
[constraints](./functions-and-generics#constraints) for dependent bounds,
method contracts and inference rules.
