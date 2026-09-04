---
title: Built-ins reference
description: Signatures and runtime contracts for Kinmokusei collection, result, channel, array-conversion, and task built-ins.
---

# Built-ins reference

Compiler built-ins have dedicated type rules and lower directly to predictable Go constructs. Most collection names can be shadowed by visible user declarations; `goChannel` and `closeGoChannel` are reserved.

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

## Channels

```ts
const unbuffered = goChannel[int]();
const buffered = goChannel[int](2);
closeGoChannel(buffered);
```

`goChannel[T]` accepts zero or one integer capacity and returns `GoChannel<T>`. A negative constant capacity is rejected before generation; a negative data-dependent capacity retains Go's runtime panic. `closeGoChannel` requires one bidirectional or send-capable channel. Closing a nil or already closed channel, and later send/receive behavior, retain Go semantics.

## Results

| Form | Valid context |
| --- | --- |
| `ok(value)` | Return from `Result<T>` |
| `ok()` | Return from `Result<void>` |
| `fail(error)` | Return from any `Result` function/method |
| `operation()?` | Propagate Kinmokusei result, Go `(T, error)`, or single Go `error` |

These forms implement a return effect; they do not construct a storable result object.

## Tasks

`go call()` is an expression producing `Task<T>`; `await task` consumes it and returns `T`; `detach task` consumes and discards it. `await task?` joins a `Task<Result<T>>` and propagates its operation error. These are syntax with dedicated lifecycle rules rather than ordinary first-class functions.
