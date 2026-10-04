---
title: Type-system reference
description: Precise Kinmokusei type representations, identity, assignability, generics, nullability, and special effects.
---

# Type-system reference

## Type syntax

| Family | Syntax | Representation |
| --- | --- | --- |
| Scalar | `int`, `string`, `bstring`, `boolean` | Corresponding Go scalar; text contracts remain distinct |
| Multiple results | `(T, U)` in a callable result position | Separate Go results, not a tuple value |
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

Parentheses and generic brackets disambiguate nested shapes. A fixed array type
currently requires a non-negative representable integer literal for its length,
not a constant name or expression. `Result` and `Task` have restricted positions
described below.

## Built-in correspondence

| Kinmokusei | Go | Notes |
| --- | --- | --- |
| `boolean` | `bool` | No truthiness |
| `string` | `string` | Immutable verified UTF-8 text |
| `bstring` | `string` | Immutable arbitrary bytes; runtime Go string boundary |
| `int`, `uint` | `int`, `uint` | Target machine width |
| `int8`…`int64` | same Go type | Fixed signed width |
| `byte`, `uint8` | `byte`, `uint8` | Identical aliases |
| `uint16`…`uint64` | same Go type | Fixed unsigned width |
| `float32` | `float32` | Fixed width |
| `float`, `number`, `float64` | `float64` | Identical types |
| `complex64`, `complex128` | same Go type | `complex`, `real`, `imag`, and imaginary constants |
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
const label = "hello";   // string
const values = [1, 2, 3]; // int[]
```

Empty collections, `nil`, and `null` need an expected or explicit type when no element/value establishes one. Public fields, parameters, results, and interface members remain explicit. Expected types flow into literals, generic calls, and untyped constants but never perform an implicit numeric conversion.

## Assignability

- Identical types are assignable.
- Scalar `string` may widen to `bstring`; the reverse requires checked decoding.
- Shared storage and function signatures preserve text contracts: `string[]`
  cannot become `bstring[]` by assignment or a no-copy conversion.
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
- `string(bstringValue)` and `string(byteSlice)` return `Result<string>` and
  reject invalid UTF-8. Conversion to a native defined `string` type likewise
  returns `Result<ThatType>` when the input needs validation.
- `bstring(bytes)` preserves arbitrary bytes; `string(int32Slice)` encodes
  code points, including compatible named slice types. Converting either
  string type to a byte/code-point slice uses a corresponding
  type or transparent alias, for example `alias Bytes = byte[]; Bytes(text)`.
  Byte conversions preserve raw bytes; decoding raw bytes into code points
  replaces invalid encodings with U+FFFD, and encoding invalid code points
  likewise uses U+FFFD. Checked byte-to-`string` decoding never does that. See
  [strings and Unicode](../book/types-and-values#strings-and-unicode).
- Named defined types convert to/from compatible underlying types explicitly.
- Enum conversion is explicit in both directions.
- Slice and fixed array do not convert implicitly; use `copyArray` or `viewArray` for the checked supported boundary.
- Class upcasts are implicit only along the declared inheritance chain; downcasts use `as?` or `as!`.
- Go interface assertions use `as?`/`as!` and retain Go assertion failure behavior.
  An assertion cannot establish a UTF-8 contract: assert `bstring`, then decode
  explicitly rather than asserting `string` or a verified-text-containing type.

Conversions do not implicitly change reference ownership, recursively copy
reference-bearing collection elements, or turn `null` into raw Go `nil`.
String/byte/code-point conversions copy or encode/decode their scalar content;
later slice mutations do not change an already converted string.

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
- `alias Name = T` or `alias Name<T> = U` creates a transparent alias; generic aliases are expanded in generated Go.

Finite recursive struct/defined shapes require slice, map, pointer, function, or channel indirection. Direct and fixed-array-only cycles are rejected.

Structural objects differ from native structs: their field-name/type set determines identity and declaration order is irrelevant. They are useful at Go data-transfer boundaries but cannot declare methods. Native structs and classes are nominal even when their fields match.

## Functions, interfaces, and method sets

Function identity includes parameter order/types, variadic status, and result. A variadic `(prefix: int, ...values: int[]) => int` is not assignable to a function taking one ordinary `int[]` parameter.

Interfaces are reference-bearing contracts. Kinmokusei classes name contracts explicitly with `implements`; imported Go interface connectivity is checked against the generated public method set. Value versus pointer receiver methods affect native struct method sets just as in Go. A pointer method requires addressable storage when automatic addressing is used.

Source and imported Go callables may have multiple results. A matching result
list can be destructured, forwarded by `return call();`, or expanded as the sole
argument to a matching call. An explicit `return first, second;` is also valid.
Result lists cannot be stored as tuple values or mixed with additional call
arguments. `Result<T>` remains a separate checked error effect.

## Generics

Functions, class/struct methods, classes, structs, interfaces, aliases, and
distinct defined types accept generic parameters. Bounds support `comparable`,
native `constraint` declarations, dependent parameters, and imported Go type
sets/method interfaces. Exact terms, underlying `~T` terms, unions, and
intersections are checked rather than approximated.

Generic named types require explicit full instantiation. Function and generic
method calls can infer arguments or accept a partial/full explicit prefix.
Generic classes support inherited state, virtual dispatch, abstract contracts,
and static methods. Static fields/accessors remain shared across instantiations
and cannot refer to class type parameters. Generic aliases expand transparently;
distinct native-struct definitions preserve nominal identity without inheriting
the underlying struct's methods. Native class/interface underlyings remain
unsupported for `distinct`.

## Nullability

`T | null` requires a nil-capable representation, including functions as well as
classes, pointers, slices, maps, channels and interfaces. The union does not
allocate a wrapper. Scalars, fixed arrays, native structs, structural objects,
`void` and `Result` cannot be nullable directly.

The compiler tracks maybe-null, definitely-null, and non-null facts separately from declared types. Facts join by guarantees across reachable flow and are invalidated by writes or effect boundaries that may change storage.

Raw Go `nil` remains separate and is used at the low-level Go boundary.

Narrowing applies to stable bindings and member paths only while the proof remains valid. Assignment, aliasing writes, address-taking, mutable capture, unknown effects, and control-flow joins may discard it. The declared `T | null` type does not change; only the current flow fact does.

<<< ../snippets-invalid/null-proof-invalidated.km{ts}

## Result and Task

`Result<T>` is valid only as a function/method return effect. It cannot be stored or nested. `Task<T>` is a local non-escaping single-consumption capability; it cannot appear in public signatures or storage positions.

`Result<void>` lowers to one `error`; `Result<T>` lowers to `(T, error)`. `Task<Result<T>>` retains both layers: `await` joins the worker, and the following `?` propagates the operation error. Neither form is an ordinary heap wrapper visible to Go callers.

## Decorator contexts and values

Decorator context types are compiler-provided nominal contracts, not
interchangeable structural object literals. `DecoratorContext` accepts any
supported target; target-specific contexts restrict application to a class,
field, constructor, method, getter, setter or parameter. See the
[target table](../book/structs-classes-interfaces#typed-decorators).

All context types expose this surface:

| Fields | Type and meaning |
| --- | --- |
| `kind` | `string`: target kind |
| `identity`, `classIdentity`, `baseIdentity` | `string`: target, declaring class and base identities |
| `overrideChain` | `string[]`: inherited slot identities |
| `className`, `memberName`, `parameterName` | `string`: source names |
| `parameterIndex` | `int`: zero-based parameter position, otherwise `-1` |
| `static`, `visibility` | `boolean`, `string`: member access contract |
| `valueType`, `valueIdentity` | `string`: source type description and nominal identity when applicable |
| `constructible`, `constructUnavailableReason` | `boolean`, `string`: construction availability |
| `construct` | `(arguments: DecoratorValue[]) => Result<DecoratorValue>` |
| `invocable`, `invokeUnavailableReason` | `boolean`, `string`: instance invocation availability |
| `invoke` | `(receiver: DecoratorValue, arguments: DecoratorValue[]) => Result<DecoratorValue>` |
| `staticInvocable`, `staticInvokeUnavailableReason` | `boolean`, `string`: static invocation availability |
| `invokeStatic` | `(arguments: DecoratorValue[]) => Result<DecoratorValue>` |

`DecoratorValue` is opaque apart from its readable `typeIdentity: string`.
Boxing and extraction preserve concrete source contracts, including nullable
qualifiers, numeric widths, nominal identities and nested type structure.
Extraction does not implicitly convert a concrete implementation to an
interface: box using that interface contract explicitly. Open type parameters
cannot be transported. The [decorator built-ins](./built-ins#decorator-values)
perform checked boxing and extraction.

## Type-related runtime failure

The checker rejects statically known type, range, comparability, addressability, and nullability violations. Operations whose validity depends on runtime data preserve Go behavior, including bounds panic, nil dereference, integer division by zero, send on a closed channel, and forced assertion/downcast panic. Checked lookup, checked receive, `as?`, `Result`, and nullable flow are the explicit non-panicking alternatives for their respective boundaries.
