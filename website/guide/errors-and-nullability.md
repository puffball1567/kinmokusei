# Errors and nullability

OnsenTamago separates expected operation failure, typed exceptions, and nullable references. Each has different syntax and generated behavior.

<<< ../snippets/errors-and-nullability.otm{ts}

## `Result<T>`

`Result<T>` is a function return effect, not a heap wrapper. It lowers directly to Go `(T, error)`, while `Result<void>` lowers to `error`.

- `ok(value)` returns success.
- `fail(err)` returns failure.
- Postfix `?` evaluates once and propagates a non-nil error.
- `[value, err]` explicitly splits the generated multiple results.

Use `Result<T>` for ordinary failures a caller is expected to handle.

## Typed exceptions

`throw`, ordered typed `catch` clauses, and `finally` are available for exceptional control flow. Only explicit OnsenTamago exceptions enter this mechanism; arbitrary Go/runtime panics are not silently converted into application exceptions.

## Nullable references

`T | null` marks a nil-backed reference as nullable. Access requires a control-flow proof that the value is not null.

```ts
const user = findUser(id);
if (user === null) { return "missing"; }
return user.name;
```

Assignments, aliases, address-taking, closures, calls, and loop backedges conservatively invalidate facts when they can make the earlier check stale.
