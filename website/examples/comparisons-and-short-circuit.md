---
title: Compare values and observe short-circuit evaluation
description: Use typed equality, string ordering, and boolean short-circuiting while making evaluation count observable.
---

# Compare values and observe short-circuit evaluation

Kinmokusei has TypeScript-shaped strict equality spellings, but it does not have JavaScript coercion. This recipe proves the four equality spellings and makes the evaluation boundary of `&&` and `||` visible.

## Source

<<< ../snippets/comparisons-and-short-circuit.km{ts}

## Run

```sh
keika check comparisons-and-short-circuit.km
keika run comparisons-and-short-circuit.km
```

Expected output:

```text
true true true true true false true true 1
```

## Read the result

| Expression | Result | Contract |
| --- | ---: | --- |
| `1 == 1` | `true` | Go-shaped equality is typed and non-coercive. |
| `1 === 1` | `true` | TypeScript-shaped equality has the same semantics. |
| `1 != 2`, `1 !== 2` | `true`, `true` | Both inequality spellings negate the same typed comparison. |
| `"hello" < "planet"` | `true` | Strings use Go's byte-wise lexicographic order. |
| `false && record(true)` | `false` | The right operand is skipped. |
| `true || record(false)` | `true` | The right operand is skipped. |
| `true && record(true)` | `true` | The right operand runs once. |

The final `evaluations` value is `1`, not `3`: only the third call is needed to determine its expression. Operands that do run are evaluated once from left to right.

## Type boundaries

Equality requires compatible comparable values. It never turns a string into a number or a number into a boolean. Slices, maps, and functions are not generally comparable; arrays and structs are comparable only when every contained value is comparable.

Ordering requires two values of the same ordered type. Convert numeric widths explicitly before comparison, and use a concrete nullable or nil-capable value when comparing against `null` or `nil`.

See [Operators](../reference/operators) for precedence and operand restrictions, and [Expressions and evaluation](../book/expressions) for the broader once-only evaluation contract.
