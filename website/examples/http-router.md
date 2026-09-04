---
title: Test an HTTP router without a port
description: Build a kinmokusei/http router and verify routes with Go's net/http/httptest package.
---

# Test an HTTP router without a port

`kinmokusei/http` implements `http.Handler`, so the standard Go HTTP testing tools work directly. This complete example opens no listener and needs no network access.

## Source

<<< ../snippets/http-router.km{ts}

## Run

```sh
keika check http-router.km
keika run http-router.km
```

Expected output:

```text
200 ok health
200 tamago:full user
```

`App` delegates matching to Go's `http.ServeMux`. The `{id}` path variable is available through `ctx.path("id")`; query and response APIs remain the original `net/http` behavior.

For production, return `app.handler()` from a router function and configure an ordinary `http.Server` at the process boundary. Timeouts, TLS, listener ownership, and graceful shutdown remain explicit application policy.

See the [`kinmokusei/http` API](../reference/standard-library#kinmokuseihttp-api) and [HTTP application guide](../guide/http).
