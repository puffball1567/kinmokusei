---
title: Failures, results, and exceptions
description: Separate expected Result failures, raw Go errors, typed exceptions, runtime panics, and nullable absence.
---

# Failures, results, and exceptions

Kinmokusei does not compress every failure into one mechanism. Choose based on whether failure is an expected return, exceptional control transfer, runtime invariant violation, or ordinary absence.

## Raw Go errors

Imported Go functions expose every result:

```ts
const [value, err] = strconv.Atoi(text);
if (err != nil) {
  console(err.Error());
  return;
}
```

This is the lowest-level boundary. Nothing wraps or discards the error. Use it when local code must inspect, combine, log, or recover without changing the function's return type.

## `Result<T>`

Use `Result<T>` when failure is an expected part of the caller contract:

```ts
function parsePort(text: string): Result<int> {
  const port = strconv.Atoi(text)?;
  if (port < 1 || port > 65535) {
    return fail(errors.New("port out of range"));
  }
  return ok(port);
}
```

The return effect lowers directly to Go `(T, error)`. `Result<void>` lowers to one `error`.

- `ok(value)` completes successfully.
- `ok()` is the void success form.
- `fail(err)` supplies the result zero value plus error.
- `operation()?` evaluates once and returns early on non-nil error.

`Result` is not a storable sum object. It cannot be a field, parameter, local value, collection element, or nested result.

On the development branch, [functions returning Result](./functions-and-generics#result-returning-function-values)
are ordinary values and can occupy those storage positions. Calling one still
requires handling its Result; simply discarding the call is an error:

<<< ../snippets-invalid/unhandled-result-function.km{ts}

### Handling and deliberate discard

On the development branch, splitting a Result into a named local error and
never using that binding is also a compile error:

<<< ../snippets-invalid/unused-result-error.km{ts}

Inspect the error, return it, or pass it to another function. When ignoring
failure is intentional, use `_` explicitly:

<<< ../snippets/result-discard.km{ts}

Inside functions, `const _ = operation()`, `let _ = operation()`, and
`_ = operation()` discard every returned value. The operation still runs
exactly once, including side effects and panics. `_` introduces no variable.
`const [value, _] = operation()` keeps the value while explicitly dropping the
error. `const _ = operation()?` instead drops only the success value and
**propagates** failure from the enclosing Result function.

This is a binding-use check, not a proof of correct recovery on every path.
It also covers Result assignment into existing local bindings, but does not
track individual writes, aliases, or whether a captured closure is called.
Passing the error onward counts as use. Ordinary imported Go `error` results
keep their existing policy; annotate a function value with `Result<T>` to
establish the checked source contract. `Task` values still require `await` or
`detach` and cannot be discarded with `_`.

## Propagating Go operations

Postfix `?` works on a compatible Go `(T, error)` or single `error` call inside a `Result` function:

```ts
function readNumber(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  return ok(value);
}
```

The non-void value initializes one binding. The control-flow boundary is visible and cannot be buried arbitrarily inside a larger expression.

## Typed exceptions

Define an exception class when an exceptional condition must cross several frames and typed recovery is useful:

```ts
class NotFoundException extends Exception {
  constructor(message: string) { super(message); }
}

function load(): void {
  throw new NotFoundException("missing");
}
```

`Exception` implements Go's error contract. `throw` accepts error-compatible values, including raw Go errors.

## Catch ordering

```ts
try {
  load();
} catch (err: NotFoundException) {
  recoverMissing(err.message);
} catch (err: Exception) {
  recoverKnown(err.message);
} catch (err: error) {
  recoverAny(err);
}
```

Clauses match source order. Specific types belong before base/general types. The checker rejects a clause already covered by an earlier catch.

<<< ../snippets-invalid/unreachable-catch.km{ts}

The catch binding is immutable and scoped to its body. Bare `throw;` rethrows the current caught value without replacing it.

## `finally`

```ts
try {
  return calculate();
} finally {
  cleanup();
}
```

Finally runs after normal completion, return, caught/rethrown language exceptions, and ordinary Go/runtime panic. If finally itself returns or throws, it replaces the earlier completion.

## Panics stay panics

Bounds errors, nil dereference, failed forced assertions, division by zero, send on closed channel, and arbitrary Go panic retain runtime panic behavior. They execute `finally` while unwinding but do not silently become a typed catch value.

This separation keeps programmer/runtime faults from being mistaken for ordinary application failure.

## Nullable absence is not an error

```ts
function display(user: User | null): string {
  if (user === null) { return "guest"; }
  return user.name;
}
```

Use nullable values when absence is an ordinary state and no explanation is required. Use `Result<User | null>` when both operation failure and a successful “not found” result are distinct states.

Keep expected failure in function signatures, exceptional recovery at an explicit `catch` boundary, and ordinary absence in the value type. [Concurrency and tasks](./concurrency-and-tasks) shows how those completion modes behave when work crosses a goroutine boundary.
