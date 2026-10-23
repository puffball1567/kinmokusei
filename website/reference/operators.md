---
title: Operators
description: Kinmokusei operator precedence, associativity, operand types, update rules, and failure behavior.
---

# Operators

## Expression binding order

Calls, member selection, indexing, slicing and postfix non-null `!` chain on
their target before prefix operators. Thus `value!.read()` checks `value`
before calling `read`, and `!callback!()` negates the checked callback's result.
Prefix `!`, `+`, `-`, `^`, `*`, `&`, `<-`, `go`, and `await` bind before type
assertions (`as? T`, `as! T`) and result propagation (`?`). Binary expressions
then group by the table below. Parentheses override that order; write
`(<-channel)!` to assert the received reference, not the channel itself.

## Binary precedence

From lowest to highest; every binary group is left-associative:

| Level | Operators | Operand summary |
| ---: | --- | --- |
| 1 | `\|\|` | `boolean` |
| 2 | `&&` | `boolean` |
| 3 | `==`, `!=`, `===`, `!==` | Compatible comparable values |
| 4 | `<`, `<=`, `>`, `>=` | Compatible ordered values |
| 5 | `+`, `-`, `|`, `^` | Numeric; `+` also supports compatible strings; bitwise forms require integers |
| 6 | `*`, `/`, `%`, `<<`, `>>`, `&`, `&^` | Numeric/integer as applicable |

The grouping follows the implemented Go-compatible bitwise precedence. `a + b << c` means `a + (b << c)`; `a | b + c` means `(a | b) + c`.

## Boolean and comparison operators

Prefix `!` accepts one `boolean`. `&&` and `||` accept boolean operands, evaluate left to right, and short-circuit the right operand when the result is already known.

The two equality spellings are semantic aliases: `==` and `===` both perform typed equality, while `!=` and `!==` both perform typed inequality. None performs JavaScript-style coercion. Operands must have compatible identical types and be comparable; slices, maps, and functions are not generally comparable. A concrete nil-capable or nullable counterpart is required when comparing with `nil` or `null`.

Ordering accepts two values of the same ordered type. Numeric widths and signedness are not mixed implicitly; strings use Go's byte-wise lexicographic order.

## Numeric identity

Typed binary operands normally require identical numeric types. A representable untyped constant can combine with a typed operand. Operations preserve defined-type identity where Go does. No implicit signed/unsigned or width conversion occurs.

<<< ../snippets-invalid/mixed-width-operator.km{ts}

Dynamic fixed-width arithmetic preserves Go overflow behavior. Statically known out-of-range constants are source diagnostics.

Unary `+` and `-` require numeric values. Binary `+`, `-`, `*`, and `/` require compatible numeric operands; `%` is integer-only. `+` also concatenates two compatible strings. Constant integer division or remainder by zero is rejected, while a data-dependent zero divisor retains Go's runtime panic behavior.

<<< ../snippets-invalid/constant-division-zero.km{ts}

## Shifts and bitwise operators

`&`, `|`, `^`, `&^`, `<<`, and `>>` require integers; unary `^` is integer complement. Shift operands need not share a type, and the result has the left operand's type.

Negative constant shifts and the compiler's constant-shift limit are rejected before generation. Dynamic negative shifts retain Go runtime panic behavior. Unary `&value` is address acquisition; binary `left & right` is bitwise AND.

<<< ../snippets-invalid/negative-shift.km{ts}

For a nonconstant shift such as `1 << n`, an untyped left operand takes its
integer type from the destination or a typed peer. This also permits `1.0 << n`
in an integer context: the literal is exactly representable as an integer.
Array indices, slice bounds and allocation sizes supply an `int` context.

<<< ../snippets/contextual-shifts.km{ts}

The left operand must fit that type before shifting: `300 << n` cannot produce
a `byte`, even if a particular shift count would discard the high bits. A
floating-point destination is invalid. Without an integer context,
`const value = 1.0 << n` defaults to a floating-point type and is rejected.
An already stored runtime value keeps its type; a later use cannot retroactively
change the type of its initializer.

<<< ../snippets-invalid/shift-left-overflow.km{ts}

The [numeric and bitwise recipe](../examples/numeric-operators) traces masks, complement, signed shift behavior, compound updates, and explicit width conversion in one runnable program.

## Pointer operators

| Operator | Meaning |
| --- | --- |
| `&value` | Address of an addressable value |
| `*pointer` | Dereference a pointer |

Non-addressable temporaries are rejected. Nil pointer dereference preserves Go panic behavior.

<<< ../snippets-invalid/address-of-temporary.km{ts}

## Updates

Supported update statements:

```text
+=  -=  *=  /=  %=  &=  |=  ^=  &^=  <<=  >>=  ++  --
```

Targets can be identifiers, writable fields/properties, assignable indexes, or
pointer dereferences. Property updates require an accessible getter and setter
and perform ordered read/modify/write calls, not atomic mutation. Generated
compound assignment evaluates a selector/index/pointer target once.
`++` and `--` never produce a value.

Strings support only `+=`. Remainder and bitwise updates require integers. Map indexes are writable; string indexes, methods, constants, and non-addressable temporary fields/array indexes are not.

## Assertions and conversions

Postfix `value!` checks a nullable or raw nil-capable reference once and yields
its non-null value. A nil value panics at the assertion's source filename, line
and column. This is an actual runtime check, not TypeScript's erased assertion.
An interface holding a typed nil pointer is still a non-nil interface; the check
does not inspect hidden pointees. `Result`, `Task`, scalars and structs cannot
be unwrapped this way. A newline before `!` starts a prefix operation instead.

<<< ../snippets/non-null-assertion.km{ts}

`as? Type` performs a checked Go-interface or class downcast and returns value plus `boolean`. `as! Type` is forced and panics on failure. Both evaluate the source once.

Native generic class downcasts check the source type arguments, not just their
generated Go storage. `Box<bstring>` is not `Box<string>`, even when the raw
bytes happen to be valid UTF-8. The same distinction applies inside arrays,
other generic arguments, and nullable types. A matching native construction can
round-trip through its base class; a mismatched checked cast returns `false`,
and a mismatched forced cast panics. Downcasting never validates or converts
the contents of a box.

Native calls carry private contract information through generic functions and
constructors. Public Go function and method signatures remain unchanged, but a
direct Go call supplies no proof of native-only qualifiers. Boxes constructed
through that path, or generic arguments erased by an ordinary Go method or
interface call, cannot subsequently establish a native generic downcast when
its source contract is unknown. Use a Kinmokusei factory to construct boxes
that must later be recovered by native checked downcasts. Public Go conversion
helpers retain Go projection behavior; they are not native contract validators.

Type-name calls such as `int64(value)` or `UserID(text)` are explicit conversions and follow representability, identity, and Go convertibility rules for the source/target pair.

## Result propagation

Postfix `?` accepts `Result<T>`, a compatible Go `(T, error)` result, or a single `error` inside a `Result`-returning function. It evaluates the operation once and returns early when the error is non-nil.

A non-void propagated value must directly initialize one binding; propagation cannot be buried in an arbitrary expression. A void or single-error operation may stand alone. Use [Failures, Result, and exceptions](../book/errors-results-exceptions) for the full return and cleanup contract.
