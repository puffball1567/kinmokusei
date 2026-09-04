---
title: Function semantics and generics
description: Declare functions, returns, arrows, callbacks, variadics, generic constraints, methods, and Go multiple-result boundaries.
---

# Function semantics and generics

Functions make parameter, result, and failure effects visible in their signature.

## Function declarations

```ts
function greet(name: string): string {
  return "Hello, " + name;
}
```

The general shape is:

```text
function name [<type parameters>] (parameters) : ResultType { body }
```

Parameters and results are explicit. `void` means the function produces no ordinary value. A non-void function must return on every continuing path.

## Parameters are local bindings

Parameters are initialized mutable local bindings. Reassigning a value parameter never changes the caller's binding. Shared effects come from reference-bearing values or explicit pointers.

```ts
function rename(user: User): void {
  user.name = "Aki"; // class instance is shared
}

function move(point: Point): Point {
  point.x++;
  return point; // caller receives a changed copy
}
```

## Return

```ts
function absolute(value: int): int {
  if (value < 0) { return -value; }
  return value;
}
```

`return;` is the void form. `return expression;` must be assignable to the result type. Returned structs/arrays/objects copy; slices/maps/classes/pointers retain their documented reference-bearing behavior.

## Arrow functions

```ts
const double = (value: int): int => value * 2;
const clamp = (value: int): int => {
  if (value < 0) { return 0; }
  return value;
};
```

Arrows are ordinary function values. Parameter types remain explicit; the result annotation may be inferred where the body and expected context establish it. Captured bindings use lexical scope; captures do not create a JavaScript runtime or dynamic closure environment beyond the generated Go closure.

## Callbacks

```ts
function apply(value: int, transform: (value: int) => int): int {
  return transform(value);
}

const result = apply(21, (value: int): int => value * 2);
```

Imported Go callbacks connect when the generated function shape is representable. Parameter/result types, variadic status, and named Go type identity must match.

## Variadic parameters

```ts
function sum(prefix: int, ...values: int[]): int {
  let total = prefix;
  for (const value of values) { total += value; }
  return total;
}
```

The rest parameter must be final and has slice type inside the function. Call with individual arguments or one final spread slice:

```ts
sum(10, 1, 2);
sum(10, values...);
```

Methods, constructors, interfaces, arrows, and function types use the same rule.

The expanded slice element type must match the rest parameter:

<<< ../snippets-invalid/variadic-spread-type.km{ts}

## Generic functions

```ts
function identity<T>(value: T): T { return value; }
function pair<T, U>(left: T, right: U): { left: T, right: U } {
  return { left: left, right: right };
}
```

Calls may infer all type arguments:

```ts
const value = identity("onsen");
```

Or supply a leading partial/full list:

```ts
identity<string>("onsen");
identity[string]("onsen");
pair<int>(1, "one");
```

Every uninferred parameter must be supplied. An uninstantiated generic function cannot be stored as a function value.

## Constraints

```ts
function choose<T extends comparable>(value: T, fallback: T): T {
  if (value === fallback) { return fallback; }
  return value;
}
```

`comparable` follows Go comparability, including contained array/struct fields. Slices, maps, and functions do not satisfy it. Broader user-written constraint type sets are not implemented.

## Generic named types

```ts
struct Page<T> {
  public items: T[];
  public function size(): int { return len(this.items); }
}

const page: Page<string> = Page<string> { items: ["one", "two"] };
```

Classes, structs, interfaces, and defined types may have type parameters. Named type positions require full explicit instantiation. Methods use the enclosing parameters; method-local type parameters are not supported.

Generic class inheritance and virtual/static generic class members remain unsupported in the preview.

## Multiple Go results

Imported Go functions expose their actual result list:

```ts
const [value, err] = strconv.Atoi(text);
```

There is no hidden error discard and no tuple wrapper. Bind every result or use `_` explicitly. Multiple results remain local to declaration/assignment/control forms.

## Result functions

`Result<T>` is a return effect, lowering to `(T, error)`; `Result<void>` lowers to `error`. It cannot be stored, nested, or used as a field/parameter. `ok`, `fail`, and `?` make the result paths explicit.

A result-producing function therefore advertises the effect at the boundary:

```ts
function validatePort(value: int): Result<int> {
  if (value < 1 || value > 65535) {
    return fail(errors.New("port out of range"));
  }
  return ok(value);
}
```

## Methods as functions with receivers

Class methods use implicit reference `this`. Struct methods default to a value receiver and may declare `pointer function` for shared mutation. External receiver syntax keeps a native named type's methods near other functions:

```ts
public function label(this: UserID): string {
  return "user:" + string(this);
}
```

The receiver type must be declared in the same module; this is not extension syntax for imported packages.

[Structs, classes, and interfaces](./structs-classes-interfaces) applies these call rules to value receivers, object identity, and contracts.
