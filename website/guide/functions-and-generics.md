---
title: Functions and generics
description: Define functions, arrows, callbacks, variadics, generic functions, and generic named types.
---

# Functions and generics

Functions make parameters and results explicit. Local call sites may infer generic arguments, but exported and stored shapes remain statically visible.

For the connected syntax treatment, read [Functions and generics](../book/functions-and-generics) in the Language Manual.

## Functions and return paths

```ts
function greet(name: string): string {
  return "Hello, " + name;
}

function log(message: string): void {
  console(message);
}
```

A block body with a non-`void` result must return on every continuing path. `Result<T>` is a distinct return effect covered in [Errors and nullability](./errors-and-nullability).

## Arrow functions

Arrows support expression and block bodies, function-type annotations, callbacks, and imported Go function values:

```ts
const add = (left: int, right: int): int => left + right;
const checked = (value: int): int => { return value; };
const transform: (value: string) => string = (text: string): string => text;
```

Call targets, receiver expressions, and arguments evaluate once in source order at the boundaries where order affects behavior.

## Rest parameters

Rest parameters use a TypeScript-shaped slice annotation and lower to Go variadics:

```ts
function sum(prefix: int, ...values: int[]): int {
  let total = prefix;
  for (const value of values) { total += value; }
  return total;
}

const direct = sum(10, 1, 2);
const values = [3, 4];
const expanded = sum(10, values...);
```

The rest parameter must be last. Inside the function it is a slice. Functions, methods, interfaces, arrows, function types, and constructors use the same rule.

## Generic functions

```ts
function identity<T>(value: T): T { return value; }
function second<T, U>(left: T, right: U): U { return right; }

const inferred = identity("hello");
const explicit = identity<string>(inferred);
const goShaped = identity[string](explicit);
const partial = second<int>(1, goShaped);
```

Calls may infer all arguments or provide a leading partial/full list with `<T>` or `[T]`. Repeated parameter uses must infer the same type, and every uninferred type parameter must be supplied. An uninstantiated generic function cannot be stored as a function value.

## Constraints

`extends comparable` maps to Go's `comparable` constraint:

```ts
function equal<T extends comparable>(left: T, right: T): boolean {
  return left === right;
}
```

Slices, maps, and functions do not satisfy it. Native constraint type sets beyond `comparable` are currently unsupported rather than approximated.

## Generic named types

Classes, structs, interfaces, and distinct defined types may declare parameters and must be fully instantiated in type positions:

```ts
struct Box<T> {
  public value: T;
  public function get(): T { return this.value; }
}

const box: Box<int> = Box<int> { value: 42 };
```

Methods use the enclosing type parameters. Go does not permit method-local type parameters, so Kinmokusei does not invent them. Generic class inheritance, virtual/static generic class members, and generic aliases remain unsupported in the current preview.

## Multiple results

Direct Go multiple results are locally destructured and are not first-class values:

```ts
const [value, err] = strconv.Atoi(text);
let [next, nextErr] = strconv.Atoi(other);
[next, nextErr] = strconv.Atoi(replacement);
```

No result, including `error`, is silently discarded. Use `_` explicitly when a position is intentionally ignored.

## Runnable recipes

- [Generic functions and structs](../examples/generics)
- [Variadics and slice spread](../examples/variadics)
- [Parse input with Result](../examples/result-parsing)
- [Interface polymorphism](../examples/polymorphism)
