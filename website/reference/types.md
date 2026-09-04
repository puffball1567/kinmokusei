---
title: Type-system reference
description: Precise Kinmokusei type representations, identity, assignability, generics, nullability, and special effects.
---

# Type-system reference

## Type syntax

| Family | Syntax | Representation |
| --- | --- | --- |
| Scalar | `int`, `string`, `boolean` | Corresponding Go scalar |
| Slice | `T[]` | `[]T` |
| Fixed array | `[N]T` | `[N]T` |
| Map | `Map<K, V>` | `map[K]V` |
| Pointer | `*T` | `*T` |
| Channel | `GoChannel<T>` | `chan T` |
| Send channel | `GoSendChannel<T>` | `chan<- T` |
| Receive channel | `GoReceiveChannel<T>` | `<-chan T` |
| Function | `(name: T, ...rest: U[]) => R` | Go function type |
| Structural object | `{ name: T, count: int }` | Deterministic anonymous Go struct |
| Named generic | `Box<T>`, `Lookup<K, V>` | Instantiated native named type |
| Nullable | `T \| null` | Nil-capable `T` without a wrapper |
| Result | `Result<T>` | Function return `(T, error)` effect |
| Task | `Task<T>` | Compiler-tracked local task state |

Parentheses and generic brackets disambiguate nested shapes. Fixed array length must be a non-negative representable constant. `Result` and `Task` have restricted positions described below.

## Built-in correspondence

| Kinmokusei | Go | Notes |
| --- | --- | --- |
| `boolean` | `bool` | No truthiness |
| `string` | `string` | UTF-8 bytes |
| `int`, `uint` | `int`, `uint` | Target machine width |
| `int8`…`int64` | same Go type | Fixed signed width |
| `byte`, `uint8` | `byte`, `uint8` | Identical aliases |
| `uint16`…`uint64` | same Go type | Fixed unsigned width |
| `float32` | `float32` | Fixed width |
| `float`, `number`, `float64` | `float64` | Identical types |
| `T[]` | `[]T` | Slice |
| `[N]T` | `[N]T` | Fixed array |
| `Map<K, V>` | `map[K]V` | Comparable key |
| `T \| null` | same nil-backed Go type | Checked nullable reference |
| `Result<T>` | `(T, error)` | Return effect only |

`error` and imported Go named/interface types keep their original Go package identity. Kinmokusei does not clone or structurally approximate them.

## Inference and expected types

Local initialized bindings may infer their type:

```ts
const count = 1;          // int
const label = "onsen";   // string
const values = [1, 2, 3]; // int[]
```

Empty collections, `nil`, and `null` need an expected or explicit type when no element/value establishes one. Public fields, parameters, results, and interface members remain explicit. Expected types flow into literals, generic calls, and untyped constants but never perform an implicit numeric conversion.

## Assignability

- Identical types are assignable.
- A representable untyped constant may adopt a compatible expected numeric type.
- Numeric widths and signedness do not change implicitly.
- Go named types preserve package identity and Go assignability.
- `alias Name = T` is transparent.
- `type Name = distinct T` is nominal and requires an explicit conversion except for compatible untyped constants.
- Distinct enum types are not interchangeable with their underlying runtime integer.
- Generic instantiations with different arguments are different types.

Go values use `go/types` assignability, addressability, method-set, and constraint rules where possible.

An incompatible initialized binding is rejected at its declaration:

<<< ../snippets-invalid/type-mismatch.km{ts}

## Explicit conversions

Write the destination type as a one-argument call:

```ts
const wide = int64(count);
const text = string(codePoint);
const id = UserID(raw);
const rawAgain = string(id);
```

- Numeric conversions follow Go width, signedness, truncation, and constant representability.
- `string(integer)` creates the UTF-8 encoding of one Unicode code point; use `fmt` or `strconv` for decimal formatting.
- Named defined types convert to/from compatible underlying types explicitly.
- Enum conversion is explicit in both directions.
- Slice and fixed array do not convert implicitly; use `copyArray` or `viewArray` for the checked supported boundary.
- Class upcasts are implicit only along the declared inheritance chain; downcasts use `as?` or `as!`.
- Go interface assertions use `as?`/`as!` and retain Go assertion failure behavior.

No conversion changes reference ownership, deep-copies a collection, or turns `null` into raw Go `nil` implicitly.

A defined type does not accept its underlying runtime type without that explicit conversion:

<<< ../snippets-invalid/defined-type-mismatch.km{ts}

## Copy and alias behavior

Scalar, fixed array, native struct, enum, and structural object assignment copies the value. A copied struct/object is shallow: a nested slice, map, pointer, channel, or class continues to share its referenced storage.

Slice assignment copies a header and aliases backing storage. Map/channel assignment copies a descriptor for the same runtime object. Class assignment copies a reference and preserves identity. Pointer assignment copies an address.

Address acquisition with `&` requires a named binding, an addressable field/index, or dereferenced pointer. Call results, conversion results, map indexes, string indexes, and other temporary values are not addressable. Pointer receiver methods likewise require a pointer or an addressable value from which the compiler can take one.

## Zero values and absence

Generated code preserves the corresponding Go zero value: numeric zero, `false`, empty string, zeroed array/struct/object, and nil for nil-backed reference shapes. Source-level construction rules may require explicit initialization even when Go has a zero value—for example, every non-null class field must be initialized on every completing constructor path.

| Form | Use |
| --- | --- |
| `null` | Checked absence for a declared `T \| null` |
| `nil` | Raw Go nil at nil-capable low-level boundaries |
| Missing map key | Returns the value type's zero value; checked lookup also returns `false` |
| Closed channel receive | Returns the element zero value; checked receive also returns `false` |

`null === null` without a concrete nullable context is rejected because it supplies no base type. A nullable value must be narrowed before member access, indexing, slicing, calling, dereference, or channel operations that require a concrete non-null receiver.

## Comparability

Scalars, pointers, channels, class references, interfaces, and enums are comparable where the corresponding Go value is comparable. Arrays and structs are comparable only when every contained field/element is comparable. Slices, maps, and functions are not comparable except for permitted nil/null checks at their boundary.

Map keys and `T extends comparable` use the same constraint.

## Collections

Fixed array length is part of identity. No implicit array/slice conversion exists. Slice bounds use Go byte/index rules and aliasing; static invalid bounds are rejected and dynamic invalid bounds panic.

Map lookup with one value returns the zero value for a missing key. Two-name binding or reassignment yields `[value, present]`, evaluating map and key once.

Slices support two-index and full three-index slicing. The result aliases the original backing array. `append` may reuse or replace that array exactly as Go does; use the returned slice. Map iteration order is unspecified. Channel operations retain blocking, close, nil-channel, and panic behavior.

## Native named types

- `struct Name` creates a nominal Go-style value type.
- `class Name` creates a pointer-backed reference type.
- `interface Name` creates an explicitly implemented interface contract.
- `enum Name` creates a nominal integer type and typed constants.
- `type Name = distinct T` creates a Go-compatible defined type.
- `alias Name = T` creates a transparent non-generic alias.

Finite recursive struct/defined shapes require slice, map, pointer, function, or channel indirection. Direct and fixed-array-only cycles are rejected.

Structural objects differ from native structs: their field-name/type set determines identity and declaration order is irrelevant. They are useful at Go data-transfer boundaries but cannot declare methods. Native structs and classes are nominal even when their fields match.

## Functions, interfaces, and method sets

Function identity includes parameter order/types, variadic status, and result. A variadic `(prefix: int, ...values: int[]) => int` is not assignable to a function taking one ordinary `int[]` parameter.

Interfaces are reference-bearing contracts. Kinmokusei classes name contracts explicitly with `implements`; imported Go interface connectivity is checked against the generated public method set. Value versus pointer receiver methods affect native struct method sets just as in Go. A pointer method requires addressable storage when automatic addressing is used.

Direct Go multiple results are statement-local compiler shapes, not tuple types. They can initialize or assign a matching binding list but cannot be returned or stored as one value.

## Generics

Functions, classes, structs, interfaces, and distinct defined types accept parameters. `extends comparable` is implemented; broader native constraint type sets are not. Generic named types require explicit full instantiation. Function calls can infer arguments or accept a partial/full explicit prefix.

Generic aliases and method-local generic parameters are unsupported. Generic class inheritance, virtual/static generic class members, and distinct types over native class/struct/interface declarations are unsupported.

## Nullability

`T | null` requires a nil-capable representation. The union does not allocate a wrapper. Scalars, arrays, native structs, structural objects, `void`, and `Result` cannot be nullable directly.

The compiler tracks maybe-null, definitely-null, and non-null facts separately from declared types. Facts join by guarantees across reachable flow and are invalidated by writes or effect boundaries that may change storage.

Raw Go `nil` remains separate and is used at the low-level Go boundary.

Narrowing applies to stable bindings and member paths only while the proof remains valid. Assignment, aliasing writes, address-taking, mutable capture, unknown effects, and control-flow joins may discard it. The declared `T | null` type does not change; only the current flow fact does.

<<< ../snippets-invalid/null-proof-invalidated.km{ts}

## Result and Task

`Result<T>` is valid only as a function/method return effect. It cannot be stored or nested. `Task<T>` is a local non-escaping single-consumption capability; it cannot appear in public signatures or storage positions.

`Result<void>` lowers to one `error`; `Result<T>` lowers to `(T, error)`. `Task<Result<T>>` retains both layers: `await` joins the worker, and the following `?` propagates the operation error. Neither form is an ordinary heap wrapper visible to Go callers.

## Type-related runtime failure

The checker rejects statically known type, range, comparability, addressability, and nullability violations. Operations whose validity depends on runtime data preserve Go behavior, including bounds panic, nil dereference, integer division by zero, send on a closed channel, and forced assertion/downcast panic. Checked lookup, checked receive, `as?`, `Result`, and nullable flow are the explicit non-panicking alternatives for their respective boundaries.
