---
title: Fetch an HTTP response with an explicit limit
description: Test kinmokusei/http with an in-memory Go transport, bounded response ownership, headers, body text, and oversize failure.
---

# Fetch an HTTP response with an explicit limit

The embedded `kinmokusei/http` client keeps transport policy and response size visible. This example supplies an in-memory Go `http.RoundTripper`, so the complete response path executes without opening a socket or depending on an external service.

## Source

<<< ../snippets/bounded-http-fetch.km{ts}

## Run

```sh
keika check bounded-http-fetch.km
keika run bounded-http-fetch.km
```

Expected output:

```text
200 true warm hello
Xello hello
true
```

## Success path

`StaticTransport` is a Kinmokusei class that explicitly implements the imported Go `http.RoundTripper` interface. Its `roundTrip` method returns a normal `*http.Response` through `Result`, which lowers to the interface's `(*http.Response, error)` method shape.

`sendWith` uses the caller-owned `http.Client`, reads and closes the response body, clones the response headers, and returns a `Response` value. The limit is measured in response-body bytes; the five-byte body `hello` fits an `int64(5)` limit exactly.

`response.ok()` means the status is from 200 through 299. Other HTTP statuses still produce a successful `Response`; transport, request construction, body reading, cancellation, and configured-limit failures use the `Result<Response>` error path.

`response.bytes()` returns a new slice. Changing the first byte of `bodyCopy` produces `Xello`, while a later `response.text()` still returns `hello`; callers cannot mutate the stored body through the returned slice.

## Oversize path

The second call permits only four bytes. The adapter reads at most one byte beyond the configured limit, closes the body, and returns an error instead of exposing a truncated success value. The final `true` confirms that boundary without depending on the diagnostic text.

In an application, use `fetch` or `fetchLimited` for a context-bound GET. Use `send` for another caller-built request and `sendWith` when timeout, transport, or connection-pool policy belongs to an explicit `http.Client`.

See the [`kinmokusei/http` API](../reference/standard-library#kinmokuseihttp-api) for every client function and [HTTP applications](../guide/http) for server and lifecycle choices.
