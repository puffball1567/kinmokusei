---
title: Work with collections
description: Use slices, fixed arrays, shared array views, maps, range, allocation, copy, clear, minimum, and maximum.
---

# Work with collections

This recipe follows storage through a slice, an independent fixed-array copy, a pointer view, an allocated destination slice, and a map. It also exercises the collection built-ins that mutate or inspect those values.

## Generic indexing and slicing

On the development branch after v0.4.0, a common underlying collection shape
allows direct indexing and slicing inside generic functions. Returning a slice
of `S` retains `S`, including named slice types; writing through that slice
updates the original backing array. A full slice can restrict its capacity:

<<< ../snippets/generic-index-slice.km{ts}

The same rules cover fixed arrays, array pointers, strings, and map indexing
with an optional presence result. Arrays retain value-copy semantics; array
pointers share their array. Class/interface elements retain their source types
and required null checks. A map slot is writable but not addressable, and a
string byte cannot be assigned:

<<< ../snippets-invalid/generic-string-index-write.km{ts}

Mixed underlying shapes remain unsupported, including unions of arrays with
slices or strings with byte slices. Dynamic index/slice bounds retain Go panics;
constant bounds and unrepresentable numeric map keys are checked before emission.

## Project tree

```text
collections/
└── main.km
```

## Source

<<< ../snippets/collections.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
5 5 4 40 3 40 0 34 true false 2 8
hello 5
```

## What happens

1. `values` is inferred as an `int[]` slice.
2. `append(values, suffix...)` expands one compatible slice and returns the result; it does not silently reassign `values`.
3. `copyArray[[3]int](values)` creates an independent fixed-array value.
4. `viewArray[[3]int](values)` creates a pointer view. Writing `(*viewed)[1]` therefore changes `values[1]`, while `fixed[1]` remains `4`.
5. `makeSlice[int](3, 5)` separates length from capacity. `copy` reports three copied elements.
6. `clear(destination)` zeroes its elements without changing its length or capacity.
7. `copy(encoded, "hello")` copies five raw string bytes into a `byte[]` and reports the copied count.
8. Range binds each slice value and evaluates the source once.
9. `[score, present]` exposes Go's comma-ok map lookup; `delete` makes the temporary key absent again.
10. `min` and `max` inspect compatible ordered values without changing their type.

Missing and nil maps return the value type's zero value plus `false`. Map iteration order is unspecified; this example never depends on it.

Both array-conversion forms panic if the source is shorter than `N`. The view also carries ordinary pointer aliasing: keep it only while shared mutation is intentional and the backing storage remains valid.

## Generic clearing and deletion

Since v0.4.0, `clear` accepts a type parameter whose possible types
are all slices or maps. `delete` requires map types with the same key type, but
their value types may differ. Both are available in generic class methods too.

<<< ../snippets/generic-collection-mutation.km{ts}

Expected output:

```text
[1 0 0 4] 2 3
0 0
```

Clearing a slice changes its shared backing elements without changing length or
capacity; elements beyond its length remain untouched. Clearing a map removes
every entry, including NaN keys. Nil maps/slices are safe. Deletion checks the
key's type, source nullability and numeric constant range at compile time.

## Generic append and copy

Since v0.4.0, a slice constraint also supports `append` and `copy`.
`append` returns the destination's original type, including a named slice type.
`copy` returns the number of elements copied and preserves the destination's
length. Both work inside generic classes and methods.

<<< ../snippets/generic-slice-builtins.km{ts}

Expected output:

```text
1 3 [1 2 3]
2 [1 2]
12
```

Source and destination elements must match for copying or spread-append, including
their nullability and generic arguments. Adding one derived-class value to a
base-class slice is allowed and preserves virtual dispatch; copying a whole
derived-class slice into a base-class slice is not. A byte-slice constraint also
supports appending or copying bytes from a string-constrained source.

See [Types and data](../guide/types-and-data) and [Type-system reference](../reference/types).
