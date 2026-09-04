---
title: Work with numeric and bitwise operators
description: Combine integer flags, shifts, complement, updates, and explicit width conversions without implicit coercion.
---

# Work with numeric and bitwise operators

Kinmokusei follows Go's typed arithmetic model: operators do not silently widen or coerce numeric values. This recipe uses `byte`, `int32`, `int`, and `int64` at boundaries where the intended width is visible.

## Source

<<< ../snippets/numeric-operators.km{ts}

## Run

```sh
keika check numeric-operators.km
keika run numeric-operators.km
```

Expected output:

```text
3 true 2 15 -4 7 3
```

## Read the expressions

| Expression | Result | Meaning |
| --- | ---: | --- |
| `read \| write` | `3` | Set both flag bits. |
| `permissions & write !== byte(0)` | `true` | Mask the write bit, then compare it with typed zero. |
| `permissions &^ read` | `2` | Clear every bit present in `read`. |
| `^byte(240)` | `15` | Complement within the eight-bit `byte` width. |
| `int32(-8) >> 1` | `-4` | Shift a signed `int32` right using Go's signed-integer behavior. |
| `updated <<= 1; updated ^= 1;` | `7` | Mutate an assignable integer without producing an expression value. |
| `int64(permissions)` | `3` | Cross a width boundary with an explicit conversion. |

The comparison needs `byte(0)` because `permissions` has type `byte`; the conversion documents the width instead of relying on coercion. A representable untyped constant such as the initial `1` or `2` may adopt its expected type, but two already typed widths never mix implicitly.

## Precedence that matters for masks

Bitwise operators do not all share one level. Multiplication, division, shifts, `&`, and `&^` bind more tightly than addition, subtraction, `|`, and `^`. Comparison binds after both groups. Therefore:

```ts
const enabled = permissions & write !== byte(0);
```

is read as `(permissions & write) !== byte(0)`. Add parentheses when they communicate the domain operation, even when the grammar does not require them.

## Fail before generation

The documentation suite also checks these rejected forms:

```ts
return value << -1;   // negative constant shift
return left | right;  // left: int, right: int32
value /= 0;           // statically known integer zero divisor
```

A shift amount or divisor computed only at runtime cannot be rejected statically. If it is negative or zero respectively, the emitted program retains Go's panic behavior. Validate data-dependent values before the operation when panic is not part of the function's contract.

See the [operator reference](../reference/operators) for the complete precedence and operand table, and [types and values](../book/types-and-values) for numeric identities and representability.
