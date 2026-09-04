# React with Gin and Fiber

This example uses one React and TypeScript frontend with two interchangeable
Kinmokusei backends. One backend calls Gin directly and the other calls Fiber
directly through `import go`; both compile to ordinary Go.

The application is a small in-memory todo service with health, list, create,
toggle, and delete operations. The frontend can switch between the running
backends without changing its HTTP contract.

## Requirements

- The `keika` command built with Go 1.27
- Go 1.27
- Node.js 22.12 or later

The backend dependencies are pinned by `kinmokusei.toml` and `kinmokusei.lock`. The
frontend dependencies are pinned by `package.json` and `package-lock.json`.

## Run both backends

Start Gin from the backend directory:

```sh
cd examples/react-web-frameworks/backend
keika deps check
keika run gin/main.km
```

In another terminal, start Fiber from the same directory:

```sh
cd examples/react-web-frameworks/backend
keika run fiber/main.km
```

Gin listens on port 8080 and Fiber listens on port 8081. `PORT` may override
either default when running only one backend.

## Run React

```sh
cd examples/react-web-frameworks/frontend
npm ci
npm run dev
```

Open the URL printed by Vite. Its development proxy maps `/gin-api` to Gin and
`/fiber-api` to Fiber, so both services remain same-origin from the browser's
perspective.

The frontend state and API transitions are tested separately:

```sh
cd examples/react-web-frameworks/frontend
npm ci
npm run test:coverage
npm run build
```

The test matrix covers initial loading, input boundaries, create/toggle/delete,
mutation failures, retry, flicker-free backend switching, and cancellation of
stale requests. Coverage thresholds apply to the application component rather
than dependency or generated bundle code.

## Verify the backend contract

From the backend directory:

```sh
./verify.sh
```

The verification script checks both Kinmokusei roots, emits each as an
isolated Go package, and runs the HTTP contract under Go's race detector. The
expected side is an independently handwritten Go implementation; generated Go
is never reused as the oracle. The matrix covers valid CRUD sequences,
malformed JSON, empty and Unicode boundary input, invalid and missing IDs, and
concurrent creation without lost or duplicate records.

The handwritten Go packages under `contract/reference` exist only as the
differential oracle. Application authors write the frontend in TypeScript and
the backend in Kinmokusei.

After installing the frontend dependencies, the complete Vite proxy and both
generated server binaries can be exercised together from the example root:

```sh
./verify.sh
```

This smoke gate starts isolated Gin, Fiber, and Vite processes, checks both
proxied health endpoints and one real create request per backend, and always
removes its temporary binaries and logs.

## Layout

| Path | Purpose |
| --- | --- |
| `frontend/` | Shared React and TypeScript application |
| `backend/store.km` | Shared concurrent todo store and JSON data shapes |
| `backend/gin/main.km` | Gin routes and server entry point |
| `backend/fiber/main.km` | Fiber routes and server entry point |
| `backend/contract/` | Independent Go oracle and differential tests |
