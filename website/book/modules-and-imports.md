---
title: Modules and imports
description: Learn per-file module scope, explicit relative imports, Go package aliases, standard modules, cycles, and locked dependencies.
---

# Modules and imports

A Kinmokusei file is a module with its own declaration scope. File layout controls name visibility explicitly; directory proximity never creates an implicit shared namespace.

## One file, one scope

Consider two files:

```text
application/
├── main.km
└── users.km
```

Declarations in `users.km` are not visible in `main.km` until imported. Passing both files as compiler roots does not merge their source scopes.

## Relative imports

Import selected declarations with braces:

```ts
import { User, findUser } from "./users";
```

The path resolves relative to the importing file. The `.km` extension may be omitted or written explicitly. The list cannot be empty, duplicate a name, or request a declaration the target does not contain or export.

Every imported name becomes one binding in the caller's module scope. It may refer to a function, class, struct, interface, enum, defined type, alias, or top-level value supported by the compiler.

## Imports are selective

If `users.km` declares both `findUser` and `normalizeID`, this import exposes only `findUser`:

```ts
import { User, findUser } from "./users";

function load(id: string): User | null {
  return findUser(id);
}
```

Calling `normalizeID` without importing it is an undefined-name diagnostic. This is module encapsulation by explicit binding, not by filename naming convention.

## Explicit source exports (development)

Development builds let a module choose its public declarations. Prefix a named
top-level declaration with `export`, or select local declarations in a list:

<<< ../snippets/source-exports-library.km{ts}

The list can appear before or after the declarations. Functions, classes,
structs, interfaces, constraints, enums, defined types, aliases, and `const`/`let`
bindings all support declaration exports. Export names must exist in the same
file, and each name can be exported only once. Lists allow a trailing comma and
follow the ordinary semicolon-omission rules.

Once a file contains any source export, only its selected declarations can be
imported. Its functions and methods can still use its private helpers. Write
`export {}` to keep every declaration private to that source module.

For compatibility, a file with **no source exports** keeps the earlier behavior:
all its top-level declarations can be selected by an import. Adding the first
source export therefore changes that file's public surface; list every name its
callers still need. This choice is per file, including when compiling several
root files together. `export c(...)` alone does not change source visibility.

A caller can use the checked example above like this:

<<< ../snippets/source-exports-main.km{ts}

Running it prints `42`. Editor definition/hover, references, and rename include
named export lists and their imported uses.

## Source imports versus Go exports

Relative Kinmokusei imports select an available declaration by its written name, regardless of whether that name begins with a lower- or uppercase letter. Source exports control availability; named imports select the caller's bindings.

An emitted Go package follows Go's export rule instead: a top-level function, type, or value whose written name begins with an uppercase Unicode letter is visible to external Go packages. For example, `function Add(...)` emits exported `Add`, while `function add(...)` remains package-local. Class/struct member visibility is separate—write `public` for a member that belongs to the public source and generated-Go contract.

Choose uppercase top-level names only for the API you intend Go consumers to use, then test that package from an external or same-package Go test. The [testing guide](../guide/testing) provides a checked example.

Source `export` does not change the emitted Go name: `export function add(...)`
is available to Kinmokusei importers but stays package-local in Go. Conversely,
an uppercase declaration remains Go-exported even if a source export list omits
it. Source-module privacy is not a separate access barrier for Go consumers.

## Imports are not transitive

Suppose `repository.km` imports `connect` from `database.km`, and `main.km` imports `load` from `repository.km`. `main.km` receives `load`; it does not receive `connect`.

```text
database.km   -- connect
      ↑
repository.km -- imports connect, declares load
      ↑
main.km       -- imports load only
```

This prevents distant implementation details from leaking whenever an intermediate module changes its own imports.

## Duplicate private spellings across modules

Two dependency modules may each declare a local `Helper` or `normalize` without colliding. The compiler links declarations by module identity and generates stable private names independent of the checkout's absolute path.

A collision occurs only when the caller tries to bind two declarations under the same imported name or conflicts with its own top-level declaration/Go alias/built-in.

## Import cycles

Relative imports form a directed module graph. Cycles are rejected:

```text
first.km imports second.km
second.km imports first.km
```

Break a cycle by moving the shared contract/value into a third lower-level module, or by reversing dependency through an interface/callback supplied by the caller.

## Go package imports

Use a package alias and literal Go import path:

```ts
import go fmt from "fmt";
import go http from "net/http";

function main(): void {
  fmt.Println(http.StatusOK);
}
```

The alias is the qualifier for exported Go declarations. Kinmokusei loads the package for the locked/effective target and retains its original named types, functions, constants, variables, methods, interfaces, and generic information where supported.

An alias cannot use a built-in type or an existing module binding.

### Named Go imports

Development builds also allow selected Go exports without a source qualifier:

```ts
import go { Println } from "fmt"
import go { Compare } from "cmp"
import go { Duration, Second } from "time"

function main(): void {
  const delay: Duration = Duration(2) * Second
  Println(delay, Compare(3, 1))
}
```

The names must be exported Go functions, types, constants, variables, or
supported compiler-recognized Go built-ins. Their original signatures,
generic constraints, constant values, and storage identities are retained.
Generated Go uses ordinary qualified references, not copied variables,
dot imports, or wrapper functions. Package initialization is unchanged.

Named lists may have trailing commas and may accompany a package-qualified
import of the same path. Each name is visible only in its importing file;
duplicate import bindings and conflicts with module declarations are rejected.
Local bindings can shadow an imported value. Type parameters can shadow an
imported type. Go package variables remain assignable and addressable; Go
constants and function declarations do not become assignable.

The checked example is `website/snippets/named-go-imports.km` in the repository.
Editor navigation leads to the import binding, and Go export names are read-only
for rename. Dependency locking and unsafe interop policies apply identically
to both import forms.

## Compiler-managed standard modules

Reserved `kinmokusei/*` modules use the named import form:

```ts
import { App, Context, fetch } from "kinmokusei/http";
```

They are embedded, compiler-versioned Kinmokusei source modules—not remote packages. Unknown, differently cased, or noncanonical reserved paths are rejected. v0.2 implements `kinmokusei/http`.

## External Go modules

An import does not download a dependency. Project code first records and locks an exact module version:

```sh
keika install --go-module github.com/google/uuid@v1.6.0
```

Then source imports a package from that locked module:

```ts
import go uuid from "github.com/google/uuid";
```

Normal `check`, `build`, `run`, LSP, and emit operations validate and consume locked dependency state without modifying it. This separates source compilation from network-dependent graph resolution.

## Project root and target

`kinmokusei.toml` establishes project identity, Go module path/version, optional target, unsafe policy, dependencies, and replacements. `kinmokusei.lock` records the canonical resolved graph and target-sensitive module state.

Source module paths remain relative to source files. Go package availability may depend on the locked GOOS, GOARCH, build tags, CGO mode, and Go version. A package that works on the host can still be unavailable for a cross target.

## Organizing a medium project

A practical layout groups by responsibility while preserving explicit imports:

```text
service/
├── kinmokusei.toml
├── kinmokusei.lock
├── main.km
├── domain/
│   ├── user.km
│   └── account.km
├── application/
│   └── registration.km
└── transport/
    └── http.km
```

`main.km` should assemble concrete implementations. Domain modules can expose defined types, value structs, interfaces, and validation functions without importing transport details. Application modules depend on those contracts; transport modules translate external HTTP/Go values at the edge.

## Diagnosing module failures

| Diagnostic family | Check |
| --- | --- |
| Cannot load relative module | Path spelling, location, `.km`, import cycle |
| Module does not declare a name | Named import list and target declaration spelling |
| Duplicate import binding | Relative names, Go aliases, and local declarations |
| Go package load failure | Lock state, target, build tags, CGO, dependency version |
| Standard package unavailable | Exact supported `kinmokusei/*` path and compiler version |

Use `keika deps check` for project graph integrity and `keika interop audit --json package/path` for Go API connectivity.

## Complete example

[Split code across modules](../examples/modules) is a runnable two-file example. [Add an external Go module](../examples/external-go-module) covers the transactional dependency workflow.

After resolving names across files and packages, [Types and values](./types-and-values) develops the copy, aliasing, and identity model behind those declarations.
