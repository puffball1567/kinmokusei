---
title: Types and values
description: Build a precise model of scalar, collection, structural, nominal, pointer, class, interface, nullable, and special effect types.
---

# Types and values

Types describe more than allowed operations. They tell you whether assignment copies an independent value, copies a descriptor that aliases storage, or preserves object identity.

## Scalar types

```ts
const ready: boolean = true;
const name: string = "hello";
const count: int = 42;
const code: uint16 = 200;
const ratio: float64 = 0.5;
```

`boolean` has no truthy conversion. `string` contains UTF-8 bytes. Integers retain signedness and width; `int`/`uint` use the target machine width. `byte` and `uint8` are identical. `float`, `number`, and `float64` are identical; `float32` is distinct.

Runtime numeric values do not widen implicitly. Convert intentionally:

```ts
const total: int64 = int64(count);
```

`complex64` and `complex128` store complex values. Use imaginary literals or
`complex(realPart, imaginaryPart)` to construct them, and `real(value)` /
`imag(value)` to extract their components. Complex values support arithmetic
and equality, but not ordering.

### Scalar constant references

Since v0.4.0, scalar `const` references and constant operations
retain Go constant semantics through local/module bindings and source imports:

<<< ../snippets/constant-aliases.km{ts}

Untyped constants retain precision until a concrete type is needed. An explicit
type annotation stays attached to the constant and its copies; a typed
`float32` constant is not an integer index just because its value is integral.
Overflow is still rejected through a chain of constant references:

<<< ../snippets-invalid/constant-alias-overflow.km{ts}

`const` still means an immutable binding, not necessarily a compile-time value.
Function results, copies of runtime variables, and three-clause loop bindings
remain runtime values. No user function is evaluated at compile time. Scalar
compile-time constants have no address; use a `let` copy when storage is needed.

String and boolean aliases keep their constant values and explicit types too.
Untyped constants can be assigned to compatible named types. Concatenation,
boolean logic, and `!` preserve named operand types; named booleans can be used
in conditions. Generic inference prefers typed arguments over untyped constants.

<<< ../snippets/scalar-constants.km{ts}

`len` of a constant string is a constant **of type `int`**, not an untyped
integer. It counts UTF-8 bytes, so `len("温泉")` is `6`. Use an explicit
conversion when a narrower integer type is needed. Constant string index and
slice bounds are checked, including through aliases:

<<< ../snippets-invalid/scalar-string-bounds.km{ts}

String slices and `len` of runtime strings remain runtime values. Constant
evaluation does not call user functions or read mutable storage.

### Constant minimum and maximum

Since v0.4.0, `min` and `max` retain a constant result when all
arguments are constants. Untyped results keep their precision until a concrete
type is needed. Named types and explicitly typed constants keep their types;
runtime arguments do not silently change width.

<<< ../snippets/ordered-constants.km{ts}

The operands must have a compatible ordered type, including inside constrained
generic functions and classes. Typed operands require every untyped constant
argument to fit that type, even if it would not be selected. The result must also
fit its destination:

<<< ../snippets-invalid/ordered-constant-overflow.km{ts}

Calls containing runtime values remain nonconstant and evaluate every argument
once in source order. `min` and `max` are not short-circuiting. Use a `let` copy
when a compile-time result needs addressable storage.

## Slice and fixed array

```ts
const values: int[] = [1, 2, 3];
const pair: [2]int = [10, 20];
```

`T[]` is a Go slice: assignment copies its header and shares backing storage. `[N]T` is a fixed value: length is part of its type and assignment copies all elements.

Since v0.4.0, `len` and `cap` of a fixed array or array pointer can
be typed `int` constants even when the array is mutable. When the argument has
no runtime calls or channel receives, it is not evaluated. In particular, a nil
array pointer can supply its type's length without being dereferenced:

<<< ../snippets/array-length-constants.km{ts}

Constant length aliases participate in bounds checks:

<<< ../snippets-invalid/array-length-bounds.km{ts}

Calls and channel receives inside the argument keep it nonconstant. No user
function is evaluated at compile time. Use `let size = len(array)` if the result
needs addressable storage.

Since v0.4.2, the same constant rules apply to
`*[N]T | null` and nullable named array pointers. The pointer may be null, but
its length is still `N`; `len(pointer) > 0` does **not** prove it is non-null.
Indexing or dereferencing still requires a null check. A `const` initialized
with such a constant length is no longer addressable; use `let` for storage.
Nullable slices, maps and channels remain runtime values whose length can be
zero. Calls returning array pointers and channel receives remain evaluated.

Generic type sets can use `len` or `cap` when every member supports that operation.
These calls remain runtime values, even for a constraint such as `~[3]int`.
A concrete array shape `[3]T`, in contrast, has a constant length. Some
constant intrinsics, such as target-dependent layout operations, remain further work.

```ts
let copiedPair = pair;
copiedPair[0] = 99; // pair[0] remains 10

const alias = values;
alias[0] = 99; // values[0] is now 99
```

No implicit array/slice conversion exists. `copyArray[[N]T](slice)` copies into an independent fixed array; `viewArray[[N]T](slice)` returns a pointer view over shared storage.

Since v0.4.0, both operations accept slice-constrained type parameters.
The target still needs a concrete array shape, possibly containing an
element type parameter or using a named array type:

<<< ../snippets/generic-array-conversion.km{ts}

The copy is shallow: class references and nested slices/maps retain their shared
objects, although replacing a copied array slot does not replace the source
slot. A view shares the slots themselves. A source shorter than the target
length panics even with spare capacity. A zero-length copy always succeeds;
a zero-length view is nil exactly when the source slice is nil.

Element types must match exactly, including nullable qualifiers; conversions do
not implicitly upcast classes or remove null checks:

<<< ../snippets-invalid/array-conversion-nullability.km{ts}

## Maps

```ts
const scores: Map<string, int> = makeMap[string, int]();
scores["Aki"] = 100;
```

Map assignment shares the same runtime map. Keys must be comparable. One-result lookup returns a zero value when missing; checked lookup adds presence:

```ts
const [score, present] = scores[name];
```

## Structural objects

```ts
function payload(): { message: string, count: int } {
  return { count: 1, message: "ready" };
}
```

A structural object's field-name/type set determines identity; field order does not. It is a copied data value with deterministic generated Go field names and JSON tags. Structural objects do not declare methods and cannot be nullable directly.

## Native structs

```ts
struct Point {
  public x: int;
  public y: int;
}

const origin = Point { x: 0, y: 0 };
```

A struct is a nominal Go-style value. Assignment, parameters, and results copy the outer value. Copying is shallow: a nested slice, map, pointer, channel, or class still refers to its original storage.

Use `*Point` and `&point` when shared mutable identity should be explicit.

## Classes

```ts
class Session {
  constructor(public id: string) {}
  public function label(): string { return "session:" + this.id; }
}
```

A class value is a reference with identity. Assignment and parameter passing preserve the same instance. Classes may implement interfaces and use explicit single inheritance.

## Interfaces

```ts
interface Reader {
  function read(): string;
}
```

A Kinmokusei class names its interface contracts with `implements`. Imported Go interfaces retain Go package identity and method-set rules. Interface assertions use `as?` or `as!`; Go interface type switches narrow case bindings.

## Pointers

`*T` is the corresponding Go pointer. `&value` requires addressable storage; `*pointer` dereferences it. Pointer assignment copies an address. Nil dereference remains a runtime panic.

A named binding, an addressable field or index, and a pointer dereference provide storage whose address can be taken. A call result is temporary and does not:

```ts
const point = makePoint();
const valid: *Point = &point;
const invalid = &makePoint(); // rejected: temporary call result
```

Bind a returned value first when its lifetime should extend through a pointer. Map indexes and string indexes are also non-addressable; use an ordinary binding when you need independently addressable storage.

Pointer receiver methods can observe nil where their implementation permits it. A pointer method on a non-addressable temporary is rejected before generation.

## Enums

```ts
enum Priority: uint8 {
  Low = 1,
  Normal,
  High = 9,
}
```

An enum is a nominal integer type. The first implicit member is zero; later implicit members increment the previous value. Initializers must be integer constant expressions representable by the underlying type. Convert explicitly between enum and integer.

## Defined types and aliases

```ts
type UserID = distinct string;
alias DisplayName = string;
```

`UserID` is nominal and may have its own methods. Crossing its underlying boundary normally requires `UserID(raw)` or `string(id)`. `DisplayName` is transparent and interchangeable with `string`.

Generic defined types are supported over representable underlying shapes:

```ts
type Lookup<K extends comparable, V> = distinct Map<K, V>;
```

Generic aliases are expanded during lowering, so they do not require Go's generic alias syntax in the generated source.

## Channels and functions

`GoChannel<T>`, `GoSendChannel<T>`, and `GoReceiveChannel<T>` preserve Go channel direction. Function types name parameters, variadic status, and result:

```ts
const transform: (value: string) => string = (text: string): string => text;
const sum: (...values: int[]) => int = calculate;
```

Function values are not comparable. A variadic signature is distinct from a function accepting one ordinary slice.

## Nullable types

```ts
let user: User | null = null;
if (user === null) { return; }
console(user.name);
```

Only nil-backed types—classes, pointers, slices, maps, channels, and suitable interfaces—can use `| null`. No wrapper is allocated. Scalars, arrays, structs, structural objects, `void`, and `Result` cannot be nullable directly.

The declared type stays nullable; control flow tracks separate definitely-null/non-null/maybe-null facts. Mutation and aliases may invalidate a proof.

## Copy and identity summary

| Type | Assignment effect |
| --- | --- |
| Scalar, enum, fixed array | Copy value |
| Struct, structural object | Shallow-copy outer value |
| Slice | Copy header; share backing storage |
| Map, channel | Copy descriptor; share runtime value |
| Pointer | Copy address |
| Class | Copy reference; preserve instance identity |
| Interface | Copy interface value; preserve contained dynamic value semantics |

Once copy and identity behavior are clear, [Functions and generics](./functions-and-generics) shows how those values cross call boundaries.
