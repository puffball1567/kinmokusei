---
title: Types and values
description: Build a precise model of scalar, collection, structural, nominal, pointer, class, interface, nullable, and special effect types.
---

# Types and values

Types describe more than allowed operations. They tell you whether assignment copies an independent value, copies a descriptor that aliases storage, or preserves object identity.

## Scalar types

```ts
const ready: boolean = true;
const name: string = "onsen";
const count: int = 42;
const code: uint16 = 200;
const ratio: float64 = 0.5;
```

`boolean` has no truthy conversion. `string` contains UTF-8 bytes. Integers retain signedness and width; `int`/`uint` use the target machine width. `byte` and `uint8` are identical. `float`, `number`, and `float64` are identical; `float32` is distinct.

Runtime numeric values do not widen implicitly. Convert intentionally:

```ts
const total: int64 = int64(count);
```

## Slice and fixed array

```ts
const values: int[] = [1, 2, 3];
const pair: [2]int = [10, 20];
```

`T[]` is a Go slice: assignment copies its header and shares backing storage. `[N]T` is a fixed value: length is part of its type and assignment copies all elements.

```ts
let copiedPair = pair;
copiedPair[0] = 99; // pair[0] remains 10

const alias = values;
alias[0] = 99; // values[0] is now 99
```

No implicit array/slice conversion exists. `copyArray[[N]T](slice)` copies into an independent fixed array; `viewArray[[N]T](slice)` returns a pointer view over shared storage.

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

Generic aliases are not supported by the current minimum Go target.

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
