# HTTP source library

`fetch.otm` contains the thin OnsenTamago HTTP client and server kernel. The
client remains deliberately smaller than an Axios-style client: request
construction and transport policy stay visible Go APIs, while the adapter adds
bounded response ownership and a small response value.

```ts
import { fetch } from "ontama/http";
import go context from "context";
import go errors from "errors";

function load(url: string): Result<string> {
  const response = fetch(context.Background(), url)?;
  if (!response.ok()) {
    return fail(errors.New("unexpected HTTP status"));
  }
  return ok(response.text());
}
```

`fetch` performs a GET with the 4 MiB default response limit.
`fetchLimited` selects another limit. `send` accepts an already constructed
`*http.Request`, and `sendWith` additionally accepts a caller-owned
`*http.Client`. Non-2xx responses are successful HTTP responses; inspect
`Response.ok()` or `Response.status`. Transport, request-construction, body-read,
and limit failures use `Result<Response>`.

The response body is read and closed before return. `Response.bytes()` returns
a copy, so callers cannot mutate the stored body. Request cancellation and
deadlines come from the explicit `context.Context` used to construct the
request.

The server kernel wraps Go's `http.ServeMux` without replacing its routing
rules:

```ts
import { App, Context } from "ontama/http";
import go fmt from "fmt";

const app = new App();
app.get("/users/{id}", (ctx: Context): void => {
  fmt.Fprintf(ctx.writer, "user=%s view=%s", ctx.path("id"), ctx.query("view"));
});
```

`App.handle` accepts an explicit HTTP method; `get`, `post`, `put`, `patch`,
and `delete` are convenience methods. `App` implements `http.Handler`, and
`handler()` exposes that interface explicitly. `Context` retains the original
writer and request and provides path, query, header, request-context, and
cookie access. Method matching, `HEAD` handling, `405`/`Allow`, route conflicts,
wildcards, redirects, and concurrent serving remain standard Go `ServeMux`
behavior. Registration should finish before concurrent serving, as with Go.

JSON response helpers, middleware, a common error boundary, timeouts, and
graceful shutdown remain higher layers. The compiler embeds this source module
and resolves the exact canonical `ontama/http` package path without a separate
download or registry lookup.
