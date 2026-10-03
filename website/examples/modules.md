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

`main.km` imports the explicitly exported `Greeting` and `greeting` declarations.
`normalize` is private: trying to import it would also be rejected, not just
trying to call it without an import. Imports used by `format.km` do not become
visible transitively. Files without any source export retain legacy importability
of all top-level declarations; selective imports alone do not make helpers private.

Relative paths may include or omit `.km`. Resolution is relative to the importing file. Cycles, missing modules, missing declarations, duplicate bindings, and import/declaration conflicts are diagnosed before Go generation.

See [Modules and projects](../guide/projects-and-cli#relative-modules).
