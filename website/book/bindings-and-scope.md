---
title: Bindings and scope
description: Understand const, let, inference, assignment, shadowing, multiple results, lexical blocks, and member visibility.
---

# Bindings and scope

A binding gives a name to a value. The binding determines whether the name may be reassigned; the value's type determines whether its storage is copied, shared, or mutated through another reference.

## Declaring a binding

Every binding has an initializer. The annotation is optional when the initializer establishes one unambiguous type.

```ts
const service = "api";
const port: int = 8080;
let attempts = 0;
```

The general forms are:

```text
const name [: Type] = expression ;
let   name [: Type] = expression ;
```

Use an annotation when it documents a public boundary, selects a non-default numeric type, provides context for an empty collection, or establishes a nullable/interface type.

```ts
const bytes: byte[] = [];
let current: User | null = null;
const reader: io.Reader = strings.NewReader("hello");
```

`nil` or `null` alone cannot supply a complete inferred type.

## `const` protects the name

This is rejected because it reassigns the binding:

```ts
const count = 1;
count = 2;
```

But `const` does not make referenced storage deeply immutable:

```ts
const values = [1, 2];
values[0] = 9;

const user = new User("Aki");
user.name = "Hana";
```

The slice binding still holds the same slice header, and the class binding still holds the same class reference. Whether an operation mutates shared storage follows the [type copy/alias contract](../reference/types#copy-and-alias-behavior).

## `let` permits reassignment

`let` allows a new value assignable to the declared type:

```ts
let count: int = 1;
count = 2;
count += 3;
count++;
```

Assignment does not change the binding's type. A `User | null` variable remains nullable after receiving a `User`; flow analysis may temporarily prove its current value non-null.

## Assignment targets

An assignment target may be a mutable binding, writable field, writable index, or pointer dereference:

```ts
count = next;
user.name = "Aki";
values[index] = 42;
(*pointer).value = 7;
```

Compound assignments and `++`/`--` are statements. Their target evaluates once, which matters when it contains a call or index expression. Constants, methods, string indexes, and non-addressable temporary storage are rejected.

## Multiple-result bindings

Direct Go calls and specific language operations may produce multiple results. Bind all positions explicitly:

```ts
const [value, err] = strconv.Atoi(text);
const [item, present] = values[key];
const [next, open] = <-channel;
```

Use `_` to acknowledge a discarded position:

```ts
const [value, _] = lookup();
```

Mutable multiple bindings and later assignment are local statement forms:

```ts
let [value, err] = read();
[value, err] = readAgain();
```

They are not tuple declarations. Multiple results cannot be captured into one variable, stored in a field, or used as a general expression value. Top-level multiple binding declarations are rejected.

## Lexical block scope

Braces introduce lexical blocks. A name declared inside a block is unavailable afterward:

```ts
if (ready) {
  const message = "ready";
  console(message);
}

// message is not in scope here.
```

Functions, arrows, loop bodies, switch/select cases, catch clauses, and explicit `{ ... }` blocks each establish their own local scope. Parameter and receiver bindings begin at the callable body.

## Case and iteration bindings

Range bindings exist only inside the loop body:

```ts
for (const [index, value] of values) {
  console(fmt.Sprint(index, value));
}
```

Type-switch bindings are narrowed and case-local:

```ts
switch (input) {
  case const reader as *strings.Reader {
    return reader.Len();
  }
  default { return 0; }
}
```

Catch bindings are immutable and catch-local. A bare rethrow is valid only within that catch body's direct exception context, not inside a nested arrow.

## Shadowing and declaration identity

An inner lexical block may introduce a name that hides an outer declaration. The compiler resolves each use to its declaration identity, rather than rewriting by spelling alone.

Use shadowing sparingly: it is useful for a narrowed or transformed value, but repeated names can hide mutation mistakes. Relative imports, Go aliases, top-level declarations, built-ins, and compiler-owned names have stricter collision checks and cannot silently shadow one another in module scope.

## Module scope is not shared scope

Two source files do not see each other's declarations merely because both are passed to the compiler. A file must import the names it uses:

```ts
import { User, findUser } from "./users";
```

Imports are not transitive. If `users.km` imports `database`, callers of `users.km` do not receive `database` automatically.

## Member visibility

Class and struct fields/methods use source-level visibility:

```ts
class Account {
  constructor(public id: int, private token: string) {}
  public function authenticated(): boolean { return this.token !== ""; }
}
```

`public` is available through the module/public Go boundary where supported. `private` is restricted to the declaring type. `protected` is available to the declaring class and descendants. Generated Go capitalization is an output mapping, not the visibility rule.

## Definite initialization

Local bindings always initialize at their declaration. A class constructor must initialize each non-null class field on every completing path. A native struct literal must provide every required field. Use an explicit nullable type when absence is a valid state rather than relying on an implicit zero reference.

[Expressions and evaluation](./expressions) uses these binding rules to explain when reads, calls, conversions, and propagation execute.
