---
title: Errors and nullability
description: Use Result, raw Go errors, typed exceptions, and flow-checked nullable references without hiding failure paths.
---

# Errors and nullability

Kinmokusei separates expected operation failure, typed exceptional control flow, ordinary Go/runtime panics, and nullable references. Each path has distinct syntax and generated behavior.

For the decision-oriented chapter, read [Failures, results, and exceptions](../book/errors-results-exceptions).

<<< ../snippets/errors-and-nullability.km{ts}

Expected output:

```text
hello:21 true
```

## `Result<T>` is a return effect

`Result<T>` lowers directly to `(T, error)`; `Result<void>` lowers to `error`.

```ts
function parse(text: string): Result<int> {
  const value = strconv.Atoi(text)?;
  if (value < 0) { return fail(errors.New("negative value")); }
  return ok(value);
}
```

- `ok(value)` returns success; `ok()` is the `Result<void>` form.
- `fail(error)` returns the result type's zero value plus the error.
- Postfix `?` evaluates once and immediately propagates a non-nil error.
- Returning another Kinmokusei result of the identical type forwards it directly.

`Result<T>` is not a runtime wrapper. It cannot be stored in a variable, parameter, field, collection element, or nested result.

Function values returning `Result<T>` can be stored in those positions; the
restriction applies to the effect itself, not the callable. Calling one still
requires propagation, explicit splitting or matching return forwarding.
An unused named source-Result error binding is a compile error. Use `_` only
when discarding failure is intentional:

```ts
const [value, err] = load() // Inspect, pass on or return err.
const [fallback, _] = load() // Explicitly discard failure.
const _ = load()? // Discard success but propagate failure.
```

Whole-call `_ = load()` explicitly discards all results. These discard forms
do not consume a Task; tasks still require `await` or `detach`. See
[handling and deliberate discard](../book/errors-results-exceptions#handling-and-deliberate-discard)
for the binding-use check and its limits.

## Raw Go errors stay available

Direct Go calls expose their actual multiple results:

```ts
const [value, err] = strconv.Atoi(text);
```

No implicit conversion hides the error. Postfix `?` is the explicit bridge from a Go `(T, error)` operation or single `error` into an enclosing `Result` function. A non-void `?` operation must initialize one binding; its control-flow boundary cannot be buried in an arbitrary expression.

## Typed exceptions

```ts
try {
  perform();
} catch (err: NotFoundException) {
  recoverMissing(err.message);
} catch (err: Exception) {
  recoverGeneral();
} finally {
  cleanup();
}
```

`Exception` is an extensible built-in class implementing Go's `error` contract. `throw` accepts an `error` value. Catch clauses are tested in source order; the checker rejects a specific type already covered by an earlier base, `Exception`, or `error` catch.
Its `message` field and `.error()` text result are `bstring`, preserving raw Go
error messages. Use checked `string(err.message)` when verified text is needed.

Bare `throw;` rethrows the currently handled exception. `finally` runs after normal completion, caught or rethrown Kinmokusei exceptions, return from `try`/`catch`, and ordinary Go/runtime panic. A return in `finally` replaces the earlier completion.

Only values carrying the explicit Kinmokusei exception marker enter catch dispatch. Bounds failures and arbitrary Go panics keep unwinding after `finally`.

## Nullable references

`T | null` marks a nil-backed reference as nullable. `null` belongs to this checked type system; imported raw Go `nil` retains its low-level Go meaning. The two do not assign implicitly.

```ts
const user = findUser(id);
if (user === null) { return "missing"; }
return user.name;
```

Only nil-capable types can be nullable: classes, pointers, slices, maps,
functions, channels and suitable interfaces. Scalars, fixed arrays, structs,
structural object values, `void` and `Result` itself cannot.

A nullable function requires the same local narrowing as other references:

<<< ../snippets/nullable-function.km{ts}

This prints `42`; calling the function before checking it against `null` would
be rejected.

Member access, calls, indexing, slicing, pointer operations, and channel operations require a non-null proof. Operations that are explicitly safe on nil Go slices/maps—such as `len`, `append`, `delete`, `clear`, and range where applicable—retain that behavior.

`?` propagation from inside `try` still exits the enclosing Result function and
runs `finally`; it is not a typed exception handled by `catch`. Handler flow
checks include interrupted states, not just the normally completed try body.
Bind getter results to locals before narrowing, because each property read can
run different code and return a different value.

## Flow facts and invalidation

The declared type never changes. The checker maintains a separate fact for each program point:

```ts
let user: User | null = findUser();
if (user !== null) {
  use(user.name); // proven non-null
  user = null;
  use(user.name); // rejected: assignment invalidated the proof
}
```

Facts join conservatively across branches, switches, selects, and loop backedges. Address-taking, aliases, mutable closure capture, receiver reassignment, unknown calls, and reachable writes invalidate facts they may affect. A later check or non-null assignment can establish a new proof.

Closure bodies are checked separately from their enclosing flow. A mutable
capture needs a check inside the closure, since it can change before invocation.
An unreachable nested closure declaration does not contribute mutation effects
to the enclosing flow, but the nested body still gets its own nullable and Task
checks.

When storage is not stable, bind a snapshot:

```ts
const snapshot = this.currentUser;
if (snapshot !== null) { use(snapshot.name); }
```

Invalid nullable access is covered by an intentionally failing documentation example; the docs check verifies that the compiler rejects it with the expected diagnostic fragment.

## Runnable recipe

[Parse input with Result](../examples/result-parsing) shows a real Go `(int, error)` call, postfix propagation, application validation, success, and zero-value failure output.
