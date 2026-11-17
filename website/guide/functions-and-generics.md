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

You can also use `const main = () => { ... }`, omit arrow
parameter/result annotations when a matching callback type supplies them, and
infer block-body results. The [Manual's arrow section](../book/functions-and-generics#arrow-functions)
explains callable declarations, recursion, mutable bindings, and the generated-Go
API change.

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

Calls may infer all arguments or provide a leading partial/full list with `<T>` or `[T]`. Repeated parameter uses must infer the same type, and every uninferred type parameter must be supplied. A stored function value must have a concrete signature, supplied by explicit type arguments or a function-typed context.

Direct arrow callback parameters can also be inferred in native and Go
generic calls. See [generic callback inference](../book/functions-and-generics#generic-callback-inference)
for examples and inference boundaries.

### Instantiated function values

::: info Available since v0.4.7
Fully instantiated generic function values are released in v0.4.7.
:::

Supply all type arguments when no function-typed context is available:

```ts
function identity<T>(value: T): T { return value; }

const integerIdentity = identity<int>;
const sameIdentity = identity[int];
const callbacks: ((value: int) => int)[] = [integerIdentity, sameIdentity];
const answer = callbacks[0](42);
```

The resulting value can be returned, stored in collections, or used as a
callback. Native `string`, nullable types, `Result<T>`, multiple results and
variadics retain their checked contracts. A Result-returning function value does
not permit silently discarding the result. Concrete native non-null parameters
and results receive an invocation wrapper when needed, so Go callers also
receive the checks. Variadic elements are checked before the body; Result
payloads are checked only on success, preserving failure zeros and typed-nil
error behavior.

Imported Go functions work too:

```ts
import go { Clone } from "slices";

const cloneIntegers = Clone<int[], int>;
const copy = cloneIntegers([1, 2, 3]);
```

### Contextual function values

::: info Available since v0.4.12
A destination function type can infer all missing generic arguments.
:::

<<< ../snippets/contextual-function-values.km{ts}

This prints `7 8 9 raw [1 2 3]`. The same inference applies to assignments,
returns, typed collection/object elements, class fields and callbacks. Both
`pair<int>` and `pair[int]` can supply a leading explicit list. Generic call
arguments can use other arguments to determine a declaration callback's type,
including when the callback comes first. Checking does not reorder runtime
argument evaluation or execute the callback during inference.

Inputs and results must determine one consistent instantiation. Verified text,
nullable references, nominal types, Result and variadic contracts remain part
of that signature. If the context cannot determine a parameter, supply type
arguments or a concrete function annotation:

<<< ../snippets-invalid/contextual-function-values.km{ts}

Inferred arguments must be nameable in generated Go. An API exposing a private
Go type needs a public function-type context or wrapper rather than an inferred
private type argument. Mutually dependent generic callbacks without a concrete
context are rejected.

Ordinary `values[index]` remains indexing when `values` is a collection, even if
it shadows a generic function. Constraints are checked when the function is
instantiated. Generic instance methods remain direct-call-only.

## Constraints

`extends comparable` maps to Go's `comparable` constraint:

```ts
function equal<T extends comparable>(left: T, right: T): boolean {
  return left === right;
}
```

Slices, maps, and functions do not satisfy it. Source constraints can also name exact and underlying type sets, for example `constraint Integer = int | ~int64`. Generic constraints such as `constraint Slice<E> = ~E[]` support dependent bounds and inference. Constraint declarations are compile-time contracts, not runtime interface values.

`constraint Narrow = A & B` restricts a type parameter to types shared by both
source type sets. Use named intermediate constraints to combine unions and
intersections; a single declaration cannot mix `|` and `&`.

Ordinary Go interfaces can add method requirements:
`constraint Printable = ~int & fmt.Stringer`. Generic code can then use both
integer operators and `value.String()`. `constraint Stream = io.Reader & io.Closer`
combines method contracts without restricting the underlying type.

Since v0.4.7, native classes, structs/pointers,
source interface values and distinct types can satisfy these Go method contracts
on source generic declarations. Only compatible public instance methods count;
pointer-only struct methods require a pointer. Generic owner substitution and
dependent method-based inference preserve text and nullability contracts. See
the [manual](../book/functions-and-generics#native-values-with-go-method-contracts)
for an executable example and the remaining boundaries.

### Source interface bounds

::: info Available since v0.4.7
Direct source interface bounds are released in v0.4.7.
:::

A runtime interface can also constrain a generic parameter:

<<< ../snippets/source-interface-constraints.km{ts}

This prints `gold`. The compiler infers `E` from the implementation's method
signature, preserving verified `string` rather than erasing it to `bstring`.
Compatible public instance methods satisfy the bound structurally; an explicit
`implements` declaration is not needed for this generic call. Ordinary class
assignment to a source interface still follows its explicit `implements` rules.

Inherited/generic interfaces, source aliases, generic owners and method-local
parameters retain source signatures. Generic code uses source method names
(`value.read()`), and accessors use property syntax. Native `Result`, multiple
results, variadics and nullable parameter/result contracts remain checked.
Private, static and generic methods do not count; pointer-only struct methods
require a pointer argument. A nullable receiver must be narrowed first.

Direct bounds such as `T extends Reader<E>` are supported. Combining a source
runtime interface into `constraint C = Reader<int> & comparable` remains
unsupported. See the [manual](../book/functions-and-generics#source-interface-bounds)
for the Go-interoperability boundary.

### Anonymous interface bounds

::: info Available since v0.4.7
Source anonymous interface bounds are released in v0.4.7.
:::

Use a method-only anonymous interface when a contract is local to one generic
function, or put it in an alias when several declarations share it:

<<< ../snippets/anonymous-interface-constraints.km{ts}

This prints `gold`. The same structural matching rules apply to named and
anonymous bounds: public non-generic instance methods, exact value/pointer method
sets, invariant signatures, and dependent inference. Verified text, nullable
parameters/results, `Result`, multiple results and variadics remain checked.
Generic class/struct/defined-type parameters and generic method owners can use
these bounds too. Editor completion and argument hints retain source method
names; bound-method references support definition lookup and rename.

An imported Go method can satisfy the bound only if its source-visible signature
matches. For example, Go `String() string` matches `String(): bstring`, not
`String(): string`. Anonymous interface operands in `constraint` type-set
composition remain unsupported; use a direct `extends interface { ... }` bound.
Anonymous getter/setter or embedded-interface entries are not part of this form.

## Generic named types

Classes, structs, interfaces, and distinct defined types may declare parameters and must be fully instantiated in type positions:

```ts
struct Box<T> {
  public value: T;
  public function get(): T { return this.value; }
}

const box: Box<int> = Box<int> { value: 42 };
```

Methods can use enclosing type parameters and declare their own. A method-local generic is emitted as a typed Go helper with an explicit receiver; it cannot be virtual, override, or final. Generic class inheritance, ordinary virtual/static members on generic classes, and generic aliases are supported. Use type parameter names distinct from the enclosing class or generic method receiver type.

## Multiple results

Source callables can declare and forward separate results:

<<< ../snippets/multiple-results.km{ts}

This prints `7 true 8`. The sole call argument to `describe` expands into its
two parameters and evaluates `pair` once. A result list is not a tuple or an
array, and cannot be stored as one value. The same signatures work in methods,
arrows, interfaces and function types.

Direct Go results use the same local binding and reassignment forms:

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
