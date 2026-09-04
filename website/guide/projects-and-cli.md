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

```text
hello-api/
├── kinmokusei.toml
├── kinmokusei.lock
├── main.km
└── users.km
```

`kinmokusei.lock` and the internal `.kinmokusei/deps/` state are created by explicit dependency operations. `.kinmokusei/gen/` is compiler-managed build output and should not be committed.

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
