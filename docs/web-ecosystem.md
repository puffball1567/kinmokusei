# Web ecosystem design

## Purpose

The web ecosystem should make an ordinary production JSON backend practical without forcing a monolithic framework or hiding Go. It is layered so small libraries can use only the HTTP kernel while larger applications opt into conventions and integrated tooling.

## Product layers

```text
ontama/http          small HTTP kernel and adapters
ontama/web/*         focused validation, middleware, config, logging, testing packages
ontama/framework     optional application framework and lifecycle conventions
external adapters    databases, queues, telemetry, providers
```

- Libraries and lightweight APIs should need only `ontama/http`.
- Conventional business backends may choose `ontama/framework`.
- Every layer may descend to direct `import go` when needed.
- Optional integrations must not burden applications that do not use them.

## Laravel-inspired, not Laravel-compatible

Useful inspiration includes:

- A coherent path from project creation to server startup.
- Conventional directories/modules.
- Generators for controllers, middleware, services, models, migrations, and jobs.
- Integrated configuration, logging, validation, database, authentication, and queues.
- Short development/test feedback loops.
- Safe defaults for common decisions.
- A clear escape path to lower-level packages.

OnsenTamago should not reproduce PHP runtime behavior, global service locators, dynamic facades, or magic that obscures dependencies and generated Go.

## Responsibility split

### Compiler-managed Go adapters

Thin, typed adapters may cover:

- `net/http`, `context`, `io`
- `encoding/json`, `net/url`, `mime/multipart`
- `log/slog`
- `time`, `os`, `io/fs`
- cryptographic primitives and TLS configuration
- `database/sql`
- `net/http/httptest`

Adapters should expose predictable OnsenTamago types while retaining access to the underlying Go values.

### Official OnsenTamago packages

Routing, middleware, validation, application lifecycle, testing helpers, and similar language-level APIs should preferably be written in OnsenTamago once the language can express them. This dogfoods package/module/error/concurrency behavior.

### External integrations

Database drivers, OAuth providers, queues, telemetry exporters, object stores, and other vendor-specific dependencies should remain separate adapters with explicit versions and licenses.

## P0: framework kernel

- Request, response, and context abstractions.
- Method/path router and route groups.
- Middleware ordering and short-circuit rules.
- JSON input/output.
- Typed error boundary and consistent error responses.
- Timeouts, cancellation, graceful shutdown.
- Small dependency surface and direct access to `net/http`.

## P1: minimum practical API

### Validation and schema

- JSON DTO, query, path, header, cookie, and form validation.
- Required, length, range, pattern, and enum rules.
- Nested objects, arrays, and nullable fields.
- Multiple errors per field with machine-readable paths/codes.
- Custom validators and a validated typed result passed to handlers.

### Basic middleware

- Request ID generation/propagation.
- Access logging and panic recovery.
- Timeout and body-size limit.
- CORS and secure headers.
- Compression and ETag/conditional requests.
- Explicit real-IP/proxy trust configuration.

### Configuration and secrets

- Required/default values.
- Typed conversion for booleans, integers, durations, and URLs.
- Prefix/environment overrides.
- Startup-wide validation.
- Secret redaction and explicit test injection.

### Structured logging

- Debug/info/warn/error.
- Request/trace/route attributes.
- Grouped fields and secret redaction.
- JSON/text handlers and an in-memory test handler.

### HTTP testing

- Request builders and response recorders.
- JSON/header/cookie assertions.
- Middleware unit tests.
- Cancellation/timeout and TLS in-process servers.
- Future typed test clients derived from routes.

## P2: common business APIs

### Authentication and authorization

- Basic/Bearer parsing and secure cookies.
- Server-side sessions and one carefully selected first auth path.
- JWT signing/verification where appropriate.
- Password hashing adapter, CSRF protection.
- OAuth 2.0/OpenID Connect client.
- Role, permission, and resource authorization.

### SQL and migrations

- Pool configuration and context-aware cancellable queries.
- Parameterized queries and typed row mapping.
- Transactions/rollback and nullable columns.
- Migration runner and test transaction/database helpers.

### Outbound HTTP

- Context, timeout, cancellation.
- JSON request/response.
- Retry/backoff with method/idempotency rules and retry budgets.
- Redirect/proxy/TLS configuration and connection reuse.
- Test transport and tracing hooks.

The initial thin client and server kernel is implemented in
`stdlib/http/fetch.otm`. Outbound APIs provide context-aware GET and
arbitrary-request entry points, a configurable response-byte limit,
status/header access, and copied text/byte bodies. Inbound APIs provide a
method-pattern `App` over `http.ServeMux` and a `Context` with path, query,
header, request-context, and cookie access. The generated `App` is directly
usable as `http.Handler` from ordinary Go. Typed JSON helpers, middleware,
retry/backoff, tracing hooks, and server lifecycle remain future layers. The
compiler embeds the source and resolves its canonical `ontama/http` import
path.

### API contracts

- OpenAPI documents, operation IDs, tags, security requirements.
- Standard error schemas.
- OnsenTamago client generation and possible future TypeScript clients.
- Contract fixtures.

### Files and content

- Multipart upload and streaming download.
- Static files, content type, range requests.
- Temporary files and explicit upload limits.
- Object-storage interface.

## P3: production and distributed systems

### Observability

- Inbound/outbound HTTP and SQL spans.
- Request count, latency, status, and in-flight metrics.
- Trace-context propagation and log correlation.
- Sampling/shutdown and OpenTelemetry adapter.

### Load control

- Rate/concurrency limiters and bounded queues.
- Circuit breaker, bulkhead, retry budget.
- Early overload rejection.

### Cache

- In-process TTL/LRU.
- Cache-key builder and stampede suppression.
- Redis adapter, HTTP cache helpers, serialization/versioning.

### Background jobs

- In-process tasks and bounded worker pools.
- Scheduler/cron, retry/backoff/dead-letter.
- Graceful draining, persistent queue adapters, idempotency.

### Operational endpoints

- Liveness, readiness, startup probes.
- Dependency health and build/version information.
- Metrics.
- Authenticated/disableable debug endpoints.

## P4: optional extensions

- WebSocket, Server-Sent Events, and streaming responses.
- HTML templates and static-site generation.
- GraphQL and gRPC/Connect-style protocols.
- Localization, mail, object stores, image processing, payments.

These should follow real application demand rather than enter the kernel preemptively.

## Suggested package layout

```text
ontama/http
ontama/http/middleware
ontama/validation
ontama/config
ontama/log
ontama/testing/http
ontama/auth
ontama/sql
ontama/migrate
ontama/client/http
ontama/health
ontama/openapi
ontama/telemetry
ontama/cache
ontama/jobs
ontama/framework
```

Package boundaries should follow dependency weight and lifecycle responsibility, not merely feature count.

## Minimum production JSON API set

The current vertical slice proves direct `net/http` plus `encoding/json` with
structural object DTOs and a bounded outbound fetch adapter. The next practical
set is:

1. HTTP kernel/router.
2. JSON DTO validation.
3. Typed error boundary/response.
4. Request ID, access log, recovery, timeout, body limit, CORS.
5. Typed config.
6. HTTP testing.
7. Graceful shutdown.
8. Health/readiness.
9. One authentication approach if required.
10. One SQL driver/migration path if persistence is required.

Every item needs compile/run/race and failure tests in a single end-to-end sample.

## First application-framework slice

- Project generator and development command.
- Explicit `Application` lifecycle and service registration.
- Controllers and route groups.
- DTO validation.
- SQL transactions/migrations.
- One authentication mode.
- Structured logging and request IDs.
- Test client/database helper.
- Production build and graceful shutdown.

Dependency registration should remain explicit or compile-time generated; runtime reflection/service location should not become mandatory.

## Implementation sequence

### Web 0: kernel

Standard adapter, request/response/context, router/groups/middleware, JSON/errors, timeout/shutdown.

### Web 1: developer experience

Validation, configuration, logging, testing, and basic security middleware.

### Web 2: real application proof

One auth path, one `database/sql` driver, migrations, outbound client, health/readiness, and an end-to-end sample.

### Web 3: ecosystem

OpenAPI/client, telemetry, cache, rate limits, background jobs, and more external adapters.

### Web 3.5: application framework

Project/framework CLI, compile-time dependency graph, conventions/generators, authenticated CRUD starter, integrated dev/test/migrate/build commands, and clear generated-file ownership.

### Web 4: protocols

Evaluate WebSocket, SSE, streaming, GraphQL, and gRPC-style protocols based on evidence.

## Design constraints

- Keep kernel dependencies small.
- Prefer libraries expressible in ordinary OnsenTamago over compiler features.
- Propagate context/cancellation through SQL, clients, tasks, and telemetry.
- Put explicit bounds on bodies, headers, forms, uploads, responses, and queues.
- Avoid mandatory mutable global singletons.
- Never log secrets, tokens, cookies, or authorization headers by default.
- Specify and unit-test middleware order and short-circuit behavior.
- Do not pull unused optional integration dependencies into applications.
- Make generated Go dependencies and licenses inspectable.

## Open questions

- Validation representation: annotations, schema DSL, or both.
- How much OpenAPI generation can be derived safely from routes/types.
- First officially supported database/driver.
- Cookie session versus Bearer token for the first auth example.
- Package prefix and compiler-bundled versus separate repositories.
- OnsenTamago-only versus TypeScript client generation.
- One framework package versus a versioned package distribution.
- Explicit service registration versus annotation-driven compile-time generation.
- Framework CLI inside `ontama` versus a separate tool.

## References

- [Go 1.22 routing enhancements](https://go.dev/blog/routing-enhancements)
- [Go database access](https://go.dev/doc/database/)
- [Go `log/slog`](https://pkg.go.dev/log/slog)
- [Go `net/http/httptest`](https://pkg.go.dev/net/http/httptest)
- [Hono middleware and helpers](https://hono.dev/docs/guides/middleware)
- [Hono validation](https://hono.dev/docs/guides/validation)
- [Gin middleware](https://gin-gonic.com/en/docs/middleware/)
- [OpenTelemetry Go instrumentation](https://opentelemetry.io/docs/languages/go/instrumentation/)
