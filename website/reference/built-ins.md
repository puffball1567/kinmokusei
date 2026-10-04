---
title: Built-ins reference
description: Signatures and runtime contracts for numeric, collection, result, channel, decorator-value, and task built-ins.
---

# Built-ins reference

Compiler built-ins have dedicated type rules and lower directly to predictable Go constructs. Most collection names can be shadowed by visible user declarations; `goChannel` and `closeGoChannel` are reserved.

## Text conversions (since v0.4.6)

| Form | Result |
| --- | --- |
| `string(rawBstring)` / `string(bytes)` | `Result<string>`; reject invalid UTF-8 without replacement |
| `bstring(text)` / `bstring(bytes)` | Raw immutable bytes; byte slices are copied |
| `string(text)` | Already verified text, without revalidation |
| `string(codePoint)` / `string(int32Slice)` | UTF-8 encoding; invalid code points encode as U+FFFD |

These conversions are ordinary type-call syntax with compiler-checked contracts.
Use `?` in a Result initializer, split `[text, err]`, or forward a checked result
directly. They are distinct from decimal formatting. See
[strings and Unicode](../book/types-and-values#strings-and-unicode).

## Complex numbers

| Form | Inputs | Result |
| --- | --- | --- |
| `complex(realPart, imaginaryPart)` | Compatible floating-point operands | `complex64` for `float32`, `complex128` for `float64` |
| `real(value)` | Complex operand | Corresponding floating-point real component |
| `imag(value)` | Complex operand | Corresponding floating-point imaginary component |

Untyped numeric constants retain constant precision until a concrete type is
required. Runtime widths do not mix implicitly. Complex values support
arithmetic and equality, not ordering or remainder.

<<< ../snippets/complex-numbers.km{ts}

This prints `3 4 -7 24 (1+2i)`. Imaginary literals, explicit complex-width
conversions, named types and compatible type-set constraints use the same
checked numeric rules.

## Collection inspection

| Form | Accepted values | Result |
| --- | --- | --- |
| `len(value)` | string, array, array pointer, slice, map, channel | `int` length |
| `cap(value)` | array, array pointer, slice, channel | `int` capacity |

Nil slices, maps, and channels follow Go's `len`/`cap` behavior where the operation is defined.

`cap` is not defined for maps:

<<< ../snippets-invalid/cap-map.km{ts}

## Slice operations

```ts
const grown = append(values, item, other);
const spread = append(values, suffix...);
const copied = copy(destination, source);
```

`append` accepts a destination slice followed by zero or more compatible elements, or exactly one expanded compatible slice. Expanding a string into `byte[]` follows Go behavior. It returns the resulting slice and never reassigns the original binding implicitly.

`copy` requires a destination slice and a source slice with the same element type. A string is also a valid source for a `byte[]` destination. It returns the number of elements copied; overlap behavior matches Go. Fixed arrays are values rather than slice arguments, so slice them explicitly when shared storage is intended.

## Map operations

```ts
delete(lookup, key);
clear(lookup);
const [value, present] = lookup[key];
```

`delete` removes a key; a missing key or nil map is a no-op. `clear` removes every entry. Checked lookup evaluates the map and key exactly once and distinguishes a stored zero value from a missing key.

## Clear, minimum, and maximum

| Form | Contract |
| --- | --- |
| `clear(slice)` | Zero every element; nil is safe |
| `clear(map)` | Remove every entry; nil is safe |
| `min(first, ...rest)` | One or more operands of one ordered numeric or string type |
| `max(first, ...rest)` | One or more operands of one ordered numeric or string type |

Named Go ordered types, compatible untyped constants, NaN, signed zero, and left-to-right evaluation retain Go behavior.

## Allocation

Since v0.4.4, `make[T](...)` allocates a complete collection type, preserving
named types and generic type parameters:

| Form | Target | Result |
| --- | --- | --- |
| `make[T](length, capacity?)` | Slice | Initialized slice of type `T` |
| `make[T](capacity?)` | Map | Empty, non-nil map of type `T` |
| `make[T](capacity?)` | Channel | Empty channel of type `T`, unbuffered by default |

<<< ../snippets/collection-make.km{ts}

A constrained target must have one underlying slice or map type. Channel
alternatives must have identical element types and compatible directions.
The target itself cannot be nullable; nullable element types remain supported.
Generic class methods and imported/re-exported constraints follow the same rules.
The result is not a constant and must be used (or explicitly discarded with `_`).
Empty slices and maps are non-nil; a map capacity
is only a hint, not an initial number of entries. Channel capacity likewise
does not enqueue values. Positive known slice lengths can establish a
constructor's nonempty-range proof; map hints and channel capacities cannot.

<<< ../snippets-invalid/collection-make.km{ts}

The existing element-oriented helpers remain supported:

```ts
const values = makeSlice[int](length);
const buffered = makeSlice[int](length, capacity);
const lookup = makeMap[string, int]();
const sized = makeMap[string, int](capacity);
```

`makeSlice[T]` requires one length and accepts one capacity. `makeMap[K, V]` accepts zero or one capacity. Negative constant sizes and statically known capacity smaller than length are source diagnostics. Dynamic invalid sizes retain Go panic behavior. `K` must be comparable.

Length is evaluated before capacity, exactly once, independently of Go toolchain intrinsic lowering.

A constant capacity smaller than the slice length is rejected before Go generation:

<<< ../snippets-invalid/make-slice-capacity.km{ts}

## Slice-to-array conversion

```ts
const copied: [3]int = copyArray[[3]int](values);
const viewed: *[3]int = viewArray[[3]int](values);
```

`copyArray` returns an independent fixed-array value. `viewArray` returns a pointer view over the slice backing storage. Each takes one explicit fixed-array target type and one compatible slice. Both panic like Go when the source is shorter than the array length.

Since v0.4.3, `copyArray` also accepts an array-constrained target type
parameter. All members must be arrays with identical element types and nullable
qualifiers, although lengths may differ. The result keeps the target's identity.
`viewArray` still requires a concrete array shape.

<<< ../snippets/generic-array-target.km{ts}

A target set containing a slice is rejected before Go generation:

<<< ../snippets-invalid/generic-array-target.km{ts}

## Channels

```ts
const unbuffered = goChannel[int]();
const buffered = goChannel[int](2);
closeGoChannel(buffered);
```

`goChannel[T]` accepts zero or one integer capacity and returns `GoChannel<T>`. A negative constant capacity is rejected before generation; a negative data-dependent capacity retains Go's runtime panic. `closeGoChannel` requires one bidirectional or send-capable channel. Closing a nil or already closed channel, and later send/receive behavior, retain Go semantics.

Since v0.4.2, `closeGoChannel` also accepts a type parameter whose type set
contains only send-capable channels, including differing element types and
imported constraints. Empty/unrestricted sets, non-channels and receive-only
members are rejected. No common element type is required for closing.

## Results

| Form | Valid context |
| --- | --- |
| `ok(value)` | Return from `Result<T>` |
| `ok()` | Return from `Result<void>` |
| `fail(error)` | Return from any `Result` function/method |
| `operation()?` | Propagate Kinmokusei result, Go `(T, error)`, or single Go `error` |

These forms implement a return effect; they do not construct a storable result object.

## Decorator values

| Form | Result | Contract |
| --- | --- | --- |
| `decoratorValue(value)` | `DecoratorValue` | Box using the inferred concrete source type |
| `decoratorValue<T>(value)` | `DecoratorValue` | Check assignability and box using explicit contract `T` |
| `decoratorValueAs<T>(value)` | `Result<T>` | Extract only when the stored source contract matches `T` |

These operations support checked decorator construction/invocation and
external registries without exposing an unchecked `any`. A mismatched
extraction is a Result failure, not a numeric or interface conversion. For DI,
box an implementation as `decoratorValue<Service>(implementation)` when the
consumer expects `Service`. Open type parameters and Result/Task effects are
not payload types. See [typed decorators](../book/structs-classes-interfaces#typed-decorators)
for a runnable construction example.

## Tasks

`go call()` is an expression producing `Task<T>`; `await task` consumes it and returns `T`; `detach task` consumes and discards it. `await task?` joins a `Task<Result<T>>` and propagates its operation error. These are syntax with dedicated lifecycle rules rather than ordinary first-class functions.
