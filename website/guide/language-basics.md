# Language basics

OnsenTamago uses typed functions, TypeScript-inspired annotations, braces, and semicolons. It keeps Go's explicit control flow and runtime behavior.

<<< ../snippets/language-basics.otm{ts}

This example prints:

```text
12 complete
```

## Bindings

Use `const` when a binding will not be reassigned and `let` when it will. Both are statically typed, and an initializer usually provides the type.

```ts
const serviceName = "api";
let attempts: int = 0;
attempts += 1;
```

`const` freezes the binding, not the object behind a reference. Value/reference behavior comes from the type.

## Functions

Functions declare parameter and return types. `void` means no result.

```ts
function greet(name: string): string {
  return "Hello, " + name;
}

const twice = (value: int): int => value * 2;
```

Top-level functions may be generic and may constrain a type parameter with `extends comparable`.

## Control flow

The language supports `if`, `switch`, `while`, C-style `for`, `for-of`, `break`, `continue`, labels, and `goto`. A single `for-of` binding receives the value; use `[index, value]` or `[key, value]` when both are needed.

```ts
for (const [index, value] of values) {
  if (index === 0) { continue; }
  consume(value);
}
```

Map iteration order is unspecified, matching Go. Range sources are evaluated once.
