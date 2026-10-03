---
title: Standard modules
description: Public API reference for compiler-managed Kinmokusei modules, beginning with kinmokusei/http.
---

# Standard modules

`kinmokusei/*` is reserved for compiler-managed source modules. The implemented
module is the exact path `kinmokusei/http`, available since v0.2.0. Unknown,
differently cased, traversal-like, or otherwise noncanonical reserved paths are
rejected.

## `kinmokusei/http` API

### Import

```ts
import { App, Context, Response, fetch, fetchLimited } from "kinmokusei/http";
```

The module is embedded in the compiler and compiled as Kinmokusei source with the application. It uses ordinary `net/http`, `context`, `errors`, and `io` packages and adds no separate runtime.

### `Response`

```text
struct Response {
  public status: int;
  public headers: http.Header;

  public function ok(): boolean;
  public function header(name: bstring): bstring;
  public function text(): Result<string>;
  public function rawText(): bstring;
  public function bytes(): byte[];
}
```

`ok()` reports a status from 200 through 299. `header` returns raw Go header
bytes. `text()` validates UTF-8 and returns an error on invalid input;
`rawText()` returns the unvalidated body as immutable bytes. `bytes()` returns
a new mutable slice copy so callers cannot mutate the stored response body.

### Fetch functions

```text
function fetch(ctx: context.Context, url: string): Result<Response>;
function fetchLimited(ctx: context.Context, url: string, maxResponseBytes: int64): Result<Response>;
function send(request: *http.Request, maxResponseBytes: int64): Result<Response>;
function sendWith(client: *http.Client, request: *http.Request, maxResponseBytes: int64): Result<Response>;
```

`fetch` uses a 4 MiB default limit. `fetchLimited` builds a GET request with the supplied context. `send` uses `http.DefaultClient`; `sendWith` accepts an explicit client.

The implementation rejects nil clients/requests and negative limits, closes every received response body, reads at most limit plus one byte, reports an oversized response, and clones headers before returning. Context cancellation is the original Go `context.Context` contract.

The [bounded HTTP fetch recipe](../examples/bounded-http-fetch) exercises the exact-limit success and oversize failure paths against a local test server.

### `Context`

```text
class Context {
  constructor(public writer: http.ResponseWriter, public request: *http.Request);

  public function path(name: bstring): bstring;
  public function query(name: bstring): bstring;
  public function header(name: bstring): bstring;
  public function context(): context.Context;
  public function cookie(name: bstring): Result<*http.Cookie>;
  public function setCookie(cookie: *http.Cookie): void;
}
```

The class preserves direct access to the original writer and request. Path values use `Request.PathValue`; query/header/cookie behavior delegates to the corresponding Go APIs.

### `App`

```text
class App implements http.Handler {
  constructor();

  public function handle(method: bstring, pattern: bstring, handler: (ctx: Context) => void): void;
  public function get(pattern: bstring, handler: (ctx: Context) => void): void;
  public function post(pattern: bstring, handler: (ctx: Context) => void): void;
  public function put(pattern: bstring, handler: (ctx: Context) => void): void;
  public function patch(pattern: bstring, handler: (ctx: Context) => void): void;
  public function delete(pattern: bstring, handler: (ctx: Context) => void): void;
  public function handler(): http.Handler;
  public function serveHTTP(writer: http.ResponseWriter, request: *http.Request): void;
}
```

Routes delegate `METHOD pattern` matching and path variables to Go 1.23's `http.ServeMux`. `handler()` returns the app as the original Go interface. There is no hidden server lifecycle; serve, timeout, shutdown, TLS, and listener policy remain ordinary Go application choices.

### Concurrency

The module composes with `Task<Result<Response>>`, but it does not automatically inherit request contexts. Pass `ctx.context()` explicitly when starting request-scoped work.
