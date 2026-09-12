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

Arrows have statically checked function types. Captured bindings use lexical
scope; captures do not create a JavaScript runtime or dynamic closure environment
beyond the generated Go closure.

### Arrow definitions and main (development)

Development builds support arrow-style entry points and function definitions:

<<< ../snippets/arrow-functions.km{ts}

Running this example prints `120 42`. A module-level `const` initialized directly
with an arrow, with an inferred or unnamed function type, is a callable
declaration. It emits an ordinary Go function and can be exported with
`export const`. Its name follows the same Go capitalization rules as `function`.
`main` must have no parameters and return `void`; `const main = () => { ... }`
supplies that result context automatically.

Give a callable declaration an explicit result annotation, or a complete
function-type annotation on the binding, when using recursion, mutual recursion,
or calls before the declaration. Its body can refer to later globals. For example,
`const twice: (n: int) => int = (n) => { return n * 2 }` declares the whole signature
without repeating it on the arrow.

Callable declarations cannot be reassigned or addressed with `&`. This is a
development-version API change: Go consumers receive a function rather than an
assignable function variable. `let` bindings remain function storage that can be
reassigned. An explicit named Go function type retains its named storage/type
contract; local arrows continue to be lexical function values.

### Local recursive arrows (development)

Local `const` and `let` arrows can call themselves when the binding has a complete
function type or the arrow has explicit parameter and result types:

<<< ../snippets/local-recursive-arrows.km{ts}

This prints `120 11`. A recursive call reads the same binding as any other use:
after a `let` is reassigned, even a previously saved closure sees the replacement
through that name. `const` still rejects reassignment. Captures remain valid when
a closure is returned from its enclosing function.

A direct arrow initializer sees its own binding, shadowing an outer binding of
the same name. An arrow parameter or inner local can shadow that name in turn.
Other initializers keep their existing scope: `const n = n + 1` inside a nested
block still reads an outer `n`.

An inferred recursive result requires an annotation. Local mutual recursion and
calls to later local bindings are not supported. Declare recursive arrows in a
block before a `for` loop, not in its three-clause initializer.

### Contextual types and block results (development)

When a binding, field, callback parameter, assignment, or return position supplies
a matching function type, arrow parameters can omit their type annotations and
the result annotation can be omitted for either body form:

```ts
function apply(value: int, transform: (n: int) => int): int {
  return transform(value)
}

const result = apply(21, (n) => { return n * 2 })
```

The parameter count and rest-parameter shape must match the expected function
type. Without such a context, parameter types are explicit. An explicit arrow
annotation is still checked against the expected function type.

Without a result context, a block arrow infers its result from its returns: the
first return establishes the default value type and subsequent returns must be
assignable to it. A block with no value returns is `void`. Mixed bare/value returns,
incompatible results, and inference from `nil`/`null` require a correction or an
explicit result annotation. Non-void arrows must return on every continuing path.
Nested arrows have independent return inference; `try/finally` keeps the enclosing
arrow's inferred result and cleanup behavior.

## Callbacks

```ts
function apply(value: int, transform: (value: int) => int): int {
  return transform(value);
}

const result = apply(21, (value: int): int => value * 2);
```

### Generic callback inference (development)

Direct arrow arguments can also omit parameter types when a generic call supplies
their context, including imported Go generic functions:

<<< ../snippets/generic-callbacks.km{ts}

This prints `42 1 3`. Other arguments, explicit type arguments, callback signature
annotations, and dependent constraints supply the callback's input types. The
callback may appear before the argument that determines its input type. An
inferred callback result can determine another type parameter, including the
input type of another callback in the same call. Generic methods and variadic
callbacks use the same rules.

Ready typed callbacks take precedence over defaulting untyped numeric constants.
If a callback needs a numeric default to determine its input type, that default
unlocks checking its body. Runtime argument order and single evaluation do not
change; creating an arrow does not execute its body.

Inference does not guess parameter types from operations inside a body. If no
argument or annotation supplies an input type, or callbacks depend on each other
without an initial type, add a parameter annotation or explicit type arguments.
Constraint failures, incompatible results, numeric overflow, and unsafe nullable
captures remain errors.

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
const value = identity("hello");
```

Or supply a leading partial/full list:

```ts
identity<string>("hello");
identity[string]("hello");
pair<int>(1, "one");
```

Every uninferred parameter must be supplied. An uninstantiated generic function cannot be stored as a function value.

A value-returning body must end in a terminating statement, as required by Go.
Remove unreachable statements after a final return. A switch or select that
can exit through `break` does not by itself prove that a value is returned.

## Constraints

```ts
function choose<T extends comparable>(value: T, fallback: T): T {
  if (value === fallback) { return fallback; }
  return value;
}
```

`comparable` follows Go comparability, including contained array/struct fields. Slices, maps, and functions do not satisfy it. A source declaration such as `constraint Numeric = ~int | ~float64` names a union of permitted types. Constraint references can be reused in other unions; overlapping terms and cycles are rejected.

Generic constraints can describe collection element relationships:

```ts
constraint Slice<E> = ~E[];
function size<S extends Slice<E>, E>(values: S): int {
  let count = 0;
  for (const _ of values) { count++; }
  return count;
}
```

Bounds may refer to later parameters. Calls infer dependent parameters from typed arguments and constraints before defaulting untyped numeric constants. Mixed untyped numeric arguments select a common numeric kind, while every actual constant must remain representable in the inferred type. `T(value)` explicitly converts to an in-scope type parameter when its bound permits the conversion.

Constraints cannot be stored as runtime values. Source-written intersections and ordinary runtime-interface terms in source constraint unions remain unsupported.

## Generic named types

```ts
struct Page<T> {
  public items: T[];
  public function size(): int { return len(this.items); }
}

const page: Page<string> = Page<string> { items: ["one", "two"] };
```

Classes, structs, interfaces, and defined types may have type parameters. Named type positions require full explicit instantiation. Methods may use the enclosing parameters and introduce separate method-local parameters. Those methods lower to standalone Go helpers; they are excluded from virtual dispatch and Go interface method sets.

Generic class inheritance, virtual methods using class parameters, static methods, and generic aliases are available. Class type parameters and generic method parameters must not hide the enclosing type name in generated helper signatures. Abstract declarations, property accessors, and static fields are not implemented.

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
