---
title: Work with collections
description: Use slices, fixed arrays, shared array views, maps, range, allocation, copy, clear, minimum, and maximum.
---

# Work with collections

This recipe follows storage through a slice, an independent fixed-array copy, a pointer view, an allocated destination slice, and a map. It also exercises the collection built-ins that mutate or inspect those values.

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
onsen 5
```

## What happens

1. `values` is inferred as an `int[]` slice.
2. `append(values, suffix...)` expands one compatible slice and returns the result; it does not silently reassign `values`.
3. `copyArray[[3]int](values)` creates an independent fixed-array value.
4. `viewArray[[3]int](values)` creates a pointer view. Writing `(*viewed)[1]` therefore changes `values[1]`, while `fixed[1]` remains `4`.
5. `makeSlice[int](3, 5)` separates length from capacity. `copy` reports three copied elements.
6. `clear(destination)` zeroes its elements without changing its length or capacity.
7. `copy(encoded, "onsen")` copies five raw string bytes into a `byte[]` and reports the copied count.
8. Range binds each slice value and evaluates the source once.
9. `[score, present]` exposes Go's comma-ok map lookup; `delete` makes the temporary key absent again.
10. `min` and `max` inspect compatible ordered values without changing their type.

Missing and nil maps return the value type's zero value plus `false`. Map iteration order is unspecified; this example never depends on it.

Both array-conversion forms panic if the source is shorter than `N`. The view also carries ordinary pointer aliasing: keep it only while shared mutation is intentional and the backing storage remains valid.

See [Types and data](../guide/types-and-data) and [Type-system reference](../reference/types).
