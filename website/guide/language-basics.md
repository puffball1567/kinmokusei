---
title: Declarations and control flow
description: Learn Kinmokusei source structure, declarations, scope, visibility, and control-flow statements.
---

# Declarations and control flow

Kinmokusei uses braces, semicolons, typed declarations, and explicit control flow. The syntax is TypeScript-inspired; evaluation and generated behavior are deliberately Go-shaped.

For a connected language treatment, read [Source text](../book/source-and-lexical), [Bindings and scope](../book/bindings-and-scope), and [Control flow](../book/control-flow) in the Language Manual.

<<< ../snippets/language-basics.km{ts}

Expected output:

```text
12 complete
```

## Source files and comments

Source files use the `.km` extension and UTF-8 text. Line comments begin with `//`; block comments use `/* ... */`. Every file has an independent module scope.

Semicolons terminate declarations and simple statements. Blocks, functions, classes, structs, interfaces, and `switch` clauses use braces.

## Bindings

Use `const` for a binding that will not be reassigned and `let` for one that will:

```ts
const serviceName = "api";
let attempts: int = 0;
attempts += 1;
```

`const` protects the binding, not the storage behind a class reference, slice, map, pointer, or other reference-bearing value. Mutability and copy behavior come from the value's type.

Local bindings infer types from their initializer in Go-like cases. Parameters, results, fields, and other public boundaries use explicit types.

## Scope and visibility

Bindings are lexical and shadowing is resolved by declaration identity. Relative imports are selective and explicit; transitive imports do not expose names.

Class and struct members use `public`, `protected`, or `private` where supported. Generated Go capitalization does not define Kinmokusei visibility—the compiler resolves visibility first and then chooses deterministic Go names.

## Conditions

Conditions must be `boolean`; there is no truthy conversion.

```ts
if (ready) {
  start();
} else {
  wait();
}
```

## Loops

The language supports `while`, C-style `for`, and `for-of` range:

```ts
while (pending()) { poll(); }

for (let i = 0; i < 3; i++) { visit(i); }

for (const value of values) { consume(value); }
for (const [index, value] of values) { consumeAt(index, value); }
for (const [key, value] of lookup) { consumeEntry(key, value); }
```

The single-binding range form always receives the value. String range yields an `int` UTF-8 byte offset and an `int32` Unicode code point. Map order is unspecified. The range source evaluates once, and each range value is a copy.

## Switch

Value switches compare one subject against source-ordered cases. The subject evaluates once; case expressions stop evaluating after the first match.

```ts
switch (status) {
  case Status.Pending { queue(); }
  case Status.Running, Status.Complete { observe(); }
  default { reject(); }
}
```

Cases do not fall through implicitly. An explicit final `fallthrough;` enters the next clause without evaluating its case expressions. Type-binding switches are reserved for Go interface values and use `case const value as Type`.

## Branches, labels, and defer

`break` and `continue` may target labels. `goto` supports validated forward and backward transfer within one function. The checker rejects jumps into nested blocks, over declarations, or across lowered exception boundaries.

`defer call();` preserves Go's deferred-call behavior. The call target and arguments follow Go-compatible evaluation rules.

## Updates

Compound assignment and `++`/`--` are statements, never expressions:

```ts
counter++;
scores[key] += delta;
```

Targets execute once. Constants, methods, string indexes, and non-addressable temporary fields are rejected; map indexes remain writable.

See [Operators](../reference/operators) for precedence and operand rules.
