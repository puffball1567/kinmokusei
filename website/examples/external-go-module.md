---
title: Add an external Go module
description: Lock an exact Go module version, import its package, and keep normal builds offline.
---

# Add an external Go module

External modules are project dependencies, not implicit downloads triggered by an import.

## Create the project manifest

```toml
[project]
name = "uuid-demo"
version = "0.1.0"
go-module = "example.com/uuid-demo"
go-version = "1.23"

[go.interop]
unsafe = "deny"
```

Add and lock one exact version:

```sh
keika install --go-module github.com/google/uuid@v1.6.0
keika deps check
keika deps licenses
```

The install transaction writes `[go.dependencies]`, `kinmokusei.lock`, and compiler-managed module state. If resolution or validation fails, it restores the previous project state.

## Import the package

```ts
import go fmt from "fmt";
import go uuid from "github.com/google/uuid";

function main(): void {
  const id = uuid.New();
  fmt.Println(id.String());
}
```

Then normal operations use only the locked state:

```sh
keika check .
keika run .
```

They do not resolve a new version or edit the module graph. Use `keika deps update`, `remove`, or `lock` when you intend to change dependency state.

Before relying on an unfamiliar package surface, inspect its interop classification:

```sh
keika interop audit --json github.com/google/uuid
```

See [Modules and projects](../guide/projects-and-cli) and the [Go interop matrix](../reference/go-interop).
