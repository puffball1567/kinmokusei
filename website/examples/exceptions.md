---
title: Typed exceptions and finally
description: Match derived exceptions in order, bridge Go errors, and guarantee cleanup with finally.
---

# Typed exceptions and `finally`

Use exceptions for exceptional control flow that must cross several call frames. Use `Result<T>` for expected operation failure that callers should handle explicitly.

## Source

<<< ../snippets/exceptions.km{ts}

## Run

```sh
keika check exceptions.km
keika run exceptions.km
```

Expected output:

```text
clean onsen
clean not-found:missing
clean not-found:closed
clean error:backend unavailable
```

`GoneException` is caught by the earlier `NotFoundException` clause because it derives from it. The final `error` clause also accepts the raw Go error created by `errors.New`. `finally` executes before every return becomes visible to the caller.

Catch order is checked statically. Putting `catch (err: error)` before the typed clause is rejected as unreachable; that failure is also part of the documentation test suite.

## Rethrow without replacing the value

Inside a catch body, bare `throw;` preserves the currently caught exception:

```ts
try {
  readConfiguration();
} catch (err: ConfigException) {
  log(err.message);
  throw;
}
```

An ordinary Go/runtime panic does not become a catchable exception. It still runs `finally`, then continues unwinding.

See [Errors and nullability](../guide/errors-and-nullability) for the `Result`, Go error, exception, and panic boundary.
