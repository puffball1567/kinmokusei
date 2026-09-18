---
title: Modules and projects
description: Organize Kinmokusei modules, lock Go dependencies and targets, and use the project-aware CLI.
---

# Modules and projects

A single `.km` file can be checked or run directly. Applications that need a stable module graph, target, or external dependencies use `kinmokusei.toml` and canonical `kinmokusei.lock` project files.

Read [Modules and imports](../book/modules-and-imports) first when learning source-level file boundaries.

## Relative modules

```ts
import { User, findUser } from "./users";
```

Each source file has its own scope. Only local declarations and explicitly imported names are visible. Transitive imports do not leak names; cycles, missing files, and duplicate bindings are diagnosed before generation.

`kinmokusei/*` is reserved for compiler-managed standard modules. The implemented `kinmokusei/http` package is embedded in the compiler; unknown or noncanonical reserved paths are rejected rather than fetched.

## Project layout

Create a ready-to-run project (available since v0.4.1):

```sh
keika new app myapp
cd myapp
keika check
keika run
keika build
```

`new app` creates `main.km`, a manifest, an initial lock, a README and a
`.gitignore`. It uses the installed Go toolchain without accessing the network.
The destination must not exist, and its parent directory must already exist.

For a reusable library, use `keika new library mylib`. It creates an exported
`greet` function in `index.km`, package metadata and a matching `go.mod`.
Choose a publishing module path with
`keika new library --module example.com/greeting mylib`. Options go between the
template name and the destination. See [external source packages](./external-packages)
for local replacement and publication.

```text
hello-api/
├── kinmokusei.toml
├── kinmokusei.lock
├── main.km
└── users.km
```

`kinmokusei.lock` and the internal `.kinmokusei/deps/` state are created by explicit dependency operations or `keika new`. Commit the lock, but not `.kinmokusei/`, which contains disposable dependency and build state. After cloning on the same target, restore that state with `keika deps fetch --offline`; after changing the manifest or target, explicitly run `keika deps lock --offline` (omit `--offline` when dependency acquisition is needed).

## Manifest

```toml
[project]
name = "hello-api"
version = "0.1.0"
go-module = "example.com/hello-api"
go-version = "1.23"

[target]
goos = "linux"
goarch = "amd64"
cgo = "disabled"
tags = "production"

[go.interop]
unsafe = "deny"

[go.dependencies]
"github.com/google/uuid" = "v1.6.0"
```

All four project keys are required. Source files live below the project root; there is no `source` manifest key. The parser rejects unknown or duplicate sections/keys instead of guessing. See [Project-file reference](../reference/project-files) for the schema.

## Dependencies are deliberate

```sh
keika install --go-module github.com/google/uuid@v1.6.0
keika deps check
keika deps licenses
```

Add, update, remove, and lock operations are transactional. If resolution fails, the previous manifest, lock, and locked module state remain unchanged.

Normal `check`, `build`, `run`, `emit-go`, and LSP paths do not acquire packages or rewrite the graph. They validate and use the locked state read-only and offline.

## Targets

The lock records the resolved GOOS, GOARCH, CGO mode, tags, and Go version. Omitted target values resolve from the compiler host, not ambient overrides. Cross-build is supported; `run` rejects a non-host target before execution.

Inspect the effective target:

```sh
keika target
```

## Generated output

Build commands use `.kinmokusei/gen/` for intermediate Go modules. Use `emit-go` when you want a durable, inspectable artifact:

```sh
keika emit-go -package hello -o generated.go main.km
```

See [Generated Go](./generated-go) for publication contracts and [CLI reference](../reference/cli) for every command and option.

For a complete two-file source example, follow [Split code across modules](../examples/modules).
