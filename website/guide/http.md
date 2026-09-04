---
title: HTTP applications
description: Build HTTP clients and servers with the embedded kinmokusei/http kernel or direct Go packages.
---

# HTTP applications

Web backends exercise Kinmokusei's intended strengths together: typed data, explicit errors, direct Go packages, tasks, contexts, deterministic binaries, and independently testable behavior.

## Two layers

```text
application code
      ↓
kinmokusei/http kernel
      ↓
Go net/http, context, encoding/json
```

Applications can use the embedded `kinmokusei/http` source module or descend directly to `import go`. The kernel does not replace the Go runtime or hide the original request, writer, handler, or context.

## Direct `net/http`

```ts
import go json from "encoding/json";
import go http from "net/http";

function health(writer: http.ResponseWriter, request: *http.Request): void {
  writer.Header().Set("Content-Type", "application/json");
  json.NewEncoder(writer).Encode({ status: "ok" });
}
```

Direct interoperability is useful when a standard or external package already exposes the API shape you need. Named types, interfaces, handlers, pointers, errors, and request contexts remain Go values.

## `kinmokusei/http`

The implemented embedded module provides:

- a Go `http.ServeMux`-compatible `App`;
- method routes and path variables;
- query, header, request-context, and cookie access;
- direct `http.Handler` use;
- structured-task compatibility;
- a bounded, context-aware fetch adapter.

The router delegates method patterns and path variables to Go 1.23's `http.ServeMux`. Route, method, context, and concurrency behavior are checked against an independent Go server.

## Cancellation and limits

Pass the original Go `context.Context` explicitly to work that can block or be cancelled. The fetch adapter keeps response bounds and context visible. Automatic task-context inheritance is not implemented, so application code should not assume that `go` expressions inherit a request lifetime.

## Frameworks

The repository includes interchangeable backends built with the real Gin and Fiber Go modules. Both use the same Kinmokusei store and are tested against the same HTTP contract as independent handwritten Go servers.

See [React with Gin and Fiber](../examples/web-backend) for the complete project and verification flow.

For the embedded client, [fetch with an explicit response limit](../examples/bounded-http-fetch) shows a caller-owned Go transport, `Result<Response>`, copied body access, and oversize failure without opening a socket.

## Operational checklist

- Keep request context explicit through downstream calls.
- Bound response bodies and background work.
- Handle malformed JSON and expected errors through visible result paths.
- Test routes under concurrency and the Go race detector.
- Preserve ordinary Go shutdown, timeout, and handler behavior at the boundary.
