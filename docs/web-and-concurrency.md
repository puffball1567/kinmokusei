# HTTP interoperability and concurrency

## Why web backends first

Web services exercise the language's intended strengths together: static DTOs, explicit errors, modules, existing Go libraries, goroutines, cancellation, deterministic binaries, and operational testing. Kinmokusei preserves direct access to Go's `net/http` rather than introducing a separate runtime.

## Layers

```text
application code
      |
implemented kinmokusei/http kernel
      |
direct import go boundary
      |
Go net/http, context, encoding/json, and other Go packages
```

Applications may use the small HTTP kernel or descend directly to `import go`.

## API direction

```ts
const app = new App();

app.get("/users/:id", (ctx: Context): Result<void> => {
  const id = ctx.params.required("id");
  const task = go users.find(ctx.context, id);
  const user = await task?;
  return ctx.json(user, 200);
});

app.listen(8080);
```

The task/result syntax and the first `App`/`Context` routing kernel are
implemented. `App` delegates method patterns and path variables to Go 1.23's
`http.ServeMux`; `Context` exposes the original writer/request plus path, query,
header, request-context, and cookie access. Direct `net/http` and
`encoding/json` interop already supports executable JSON APIs. The same
embedded `kinmokusei/http` source also provides a thin fetch adapter that keeps
context and response limits explicit and composes with
`Task<Result<Response>>` without introducing an Axios-style configuration
layer.

## Concurrency model

Low-level implemented constructs map directly to Go:

- `go callback(args);`
- `defer callback(args);`
- directional `GoChannel<T>` types
- send, receive, checked receive, close, range, and `select`

The higher-level direction is structured concurrency:

```ts
const task: Task<User> = go users.load(ctx.context, id);
const user: User = await task;
```

### Task rules

- A `go` expression over an ordinary function, method, or `Result` call returns
  `Task<T>`; a raw `go call();` statement retains direct Go behavior.
- A task binding is local, non-copyable, non-reassignable, and non-escaping. It
  must be consumed exactly once with `await` or explicit `detach` on every
  continuing control-flow path.
- `await` produces the ordinary value. Postfix `?` on `await` propagates a
  `Task<Result<T>>` failure through the enclosing `Result` function.
- The callee and arguments are evaluated synchronously, once and in source
  order, before the worker goroutine starts, matching Go call-start semantics.
- Panic is contained in the task goroutine and re-panicked by `await`.
  Detaching explicitly discards the result; an unhandled detached panic remains
  fatal.

### Cancellation direction

- Context is passed explicitly through ordinary `context.Context` values today.
- Future task syntax may make context inheritance and cancellation propagation
  explicit at the language/runtime boundary.
- Ignoring cancellation requires an explicit detached lifetime.

### Shared state

- Capturing the same mutable variable from multiple tasks should warn or fail where statically visible.
- The language does not promise implicit race prevention; generated Go must remain compatible with `go test -race`.

## Channels

Raw Go channels are the interoperability substrate and retain Go's direction and type behavior.

```ts
const channel = goChannel[int](1);
channel <- 42;
const [value, open] = <-channel;
select {
  case const item = <-channel { consume(item); }
  default { idle(); }
}
closeGoChannel(channel);
```

## Performance

- Use the ordinary Go scheduler and `net/http`.
- Keep task wrappers simple and profileable.
- Avoid unnecessary reflection and dynamic boxing.
- Do not make zero allocation a universal requirement.
- Preserve a clear escape path to generated or handwritten Go for measured hotspots.
- Test throughput concerns together with correctness, cancellation, races, leaks, and shutdown behavior.
