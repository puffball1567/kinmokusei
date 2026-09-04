---
title: Split code across modules
description: Import selected declarations from a neighboring Kinmokusei source file without leaking its private helpers.
---

# Split code across modules

Every `.km` file is a module with its own scope. Import only the declarations the caller needs.

## Project tree

```text
modules/
├── format.km
└── main.km
```

## `format.km`

<<< ../snippets/modules/format.km{ts}

## `main.km`

<<< ../snippets/modules/main.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
Welcome, Aki
Welcome, guest
```

`main.km` imports `Greeting` and `greeting`; it cannot call the unimported `normalize` helper. Imports used by `format.km` would not become visible transitively either.

Relative paths may include or omit `.km`. Resolution is relative to the importing file. Cycles, missing modules, missing declarations, duplicate bindings, and import/declaration conflicts are diagnosed before Go generation.

See [Modules and projects](../guide/projects-and-cli#relative-modules).
