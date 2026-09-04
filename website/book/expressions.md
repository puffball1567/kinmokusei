---
title: Expressions and evaluation
description: Learn value expressions, literals, calls, indexing, conversion, assertions, propagation, arrows, tasks, and evaluation order.
---

# Expressions and evaluation

An expression computes a value. Kinmokusei keeps expressions composable but reserves mutation and control transfer for statements, making evaluation boundaries visible.

## Expressions versus statements

These forms are expressions and may supply a value:

```ts
1 + 2
user.name
values[index]
parse(text)
new User("Aki")
(value: int): int => value + 1
```

Assignment, compound assignment, increment/decrement, channel send, `return`, `throw`, `break`, and `continue` are statements. They cannot be nested inside another expression.

```ts
count++;          // statement
count += 2;       // statement
channel <- value; // statement
```

This means there is no accidental use of an assignment's result and no prefix/postfix increment value distinction.

## Collection and structural literals

An array literal uses its expected type or a common element type:

```ts
const values: int[] = [1, 2, 3];
const pair: [2]string = ["hot", "spring"];
```

An empty `[]` needs context. A fixed array and a slice are different types even though both use bracket literals.

Structural object literals use named fields:

```ts
const payload: { message: string, count: int } = {
  count: 1,
  message: "ready",
};
```

Field order does not affect structural object identity. Missing, extra, duplicate, and mistyped fields are diagnosed individually.

Native structs and imported Go structs use qualified nominal literals:

```ts
const point = Point { x: 1, y: 2 };
const cookie = http.Cookie{Name: "session", Value: "abc"};
```

Class instances always use `new Class(arguments...)` and a declared constructor.

## Member selection

The dot operator selects fields, methods, enum members, static class methods, and Go package declarations according to the receiver:

```ts
user.name
user.display()
Status.Ready
Factory.create()
fmt.Println("ready")
```

Visibility and method-set checks happen at the `.km` source. A nullable receiver must first have a valid non-null proof.

## Indexing and slicing

```ts
const first = values[0];
const middle = values[1:3];
const capacityBound = values[1:3:4];
```

The low, high, and maximum bounds are optional where Go permits them; a full three-index slice requires both high and max. Slice expressions share backing storage. Static invalid bounds are diagnostics, while data-dependent invalid bounds retain Go panic behavior.

String indexing returns one byte. Range over a string when you need decoded Unicode code points.

A map index has two useful forms:

```ts
const value = lookup[key];
const [value, present] = lookup[key];
```

The first returns the value type's zero value when absent. The second distinguishes absence and evaluates the map and key once.

## Calls

A call evaluates the callee/receiver and arguments once in source order before entering the callable:

```ts
const result = service.load(first(), second());
```

The observable order is:

1. evaluate `service` and resolve the call target;
2. evaluate `first()` once;
3. evaluate `second()` once;
4. invoke `load`.

This also applies when calls lower through generated adapters, virtual dispatch, `Result`, or Go interop. More broadly, selectors, index targets, map lookups, assertions, range sources, switch subjects, and task-start arguments are not duplicated by lowering: source evaluation order remains the contract.

## Generic calls

Type arguments may be inferred or supplied explicitly:

```ts
const inferred = identity("onsen");
const explicit = identity<string>("onsen");
const goShaped = identity[string]("onsen");
```

The angle form is TypeScript-shaped; the bracket form is useful when mirroring explicit Go generic calls. Partial leading arguments are accepted when the rest can be inferred.

## Variadic calls and spread

Pass individual elements or expand one final slice:

```ts
sum(10, 1, 2, 3);
sum(10, values...);
```

Only one spread is permitted, it must be the final argument, and its slice element type must match the rest parameter. Spread does not mean general iterable expansion.

## Unary and binary operators

Prefix operators include logical `!`, numeric `+`/`-`, bitwise `^`, address `&`, dereference `*`, and channel receive `<-`.

Binary operators are left-associative within their precedence group. `&&` and `||` short-circuit. Both `==`/`!=` and TypeScript-shaped `===`/`!==` use the same typed, non-coercive equality contract—there is no JavaScript loose equality conversion.

Use the [operator reference](../reference/operators) for the complete precedence table and operand restrictions.

## Explicit conversion

A type called with one argument is an explicit conversion:

```ts
const wide = int64(count);
const id = UserID(raw);
const rawAgain = string(id);
```

Conversions follow representability and Go convertibility rules. They do not deep-copy referenced storage. `string(integer)` creates a Unicode code point string, not decimal formatting.

## Checked and forced assertions

Use `as?` when failure is data and `as!` when failure is a programming invariant violation:

```ts
const [reader, ok] = value as? *strings.Reader;
const required = value as! *strings.Reader;
```

Checked assertion returns the target zero value and `false`. Forced assertion panics on failure. Class downcasts use the same spelling within one inheritance chain and preserve identity.

## Result propagation

Postfix `?` is a control-flow expression available inside a compatible `Result` function:

```ts
function port(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
```

The operand evaluates once. On error, the function returns immediately with its result zero value plus the error. On success, the expression yields the non-error value. `?` is not optional chaining and does not apply to nullable values.

## Arrow expressions

Arrows are typed function values with expression or block bodies:

```ts
const double = (value: int): int => value * 2;
const checked = (value: int): int => {
  if (value < 0) { return 0; }
  return value;
};
```

Arrow parameter types are explicit. The result annotation may be omitted when the body and expected context establish it, but writing it is recommended at stored/public boundaries. Captures obey normal lexical scope; mutable capture can invalidate nullable-flow proofs.

## Task expressions

`go call()` without a semicolon context creates a structured task expression:

```ts
const task: Task<int> = go calculate();
const value = await task;
```

The call target and arguments evaluate synchronously once before the worker starts. `await` consumes the task exactly once. A raw `go call();` statement instead launches an unmanaged Go goroutine.

[Control flow](./control-flow) builds on these evaluation rules when branches, loops, switches, and cleanup determine whether an expression runs.
