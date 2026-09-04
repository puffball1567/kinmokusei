---
title: Troubleshooting
description: Diagnose source, project-lock, Go-package, target, generated-code, and editor failures systematically.
---

# Troubleshooting

Start with the narrowest command that reproduces the problem. `check` isolates source and project validation; `build` additionally exercises Go generation and the selected target; `run` also requires a host-compatible executable.

## From one diagnostic to a fix

Use a short loop: run `check`, fix the first source-positioned rule, and run `check` again before investigating later messages. Parser recovery can report several independent problems, but an early syntax error may also make later source look different from what you intended.

This checked invalid example accesses a nullable value without a proof:

<<< ../snippets-invalid/null-access.km{ts}

Plain mode identifies the original file, one-based line and column, then the failed rule:

```text
src/main.km:6:10: nullable value User | null must be checked against null before member access
```

The fix is not a generated-Go edit. Establish a source-level fact on every continuing path:

```ts
function display(user: User | null): string {
  const current = user;
  if (current === null) { return "guest"; }
  return current.name;
}
```

If a fact later disappears, look for the assignment, aliasing write, address-taking, mutable capture, unknown call, or control-flow join named by the diagnostic. The [nullable-flow recipe](../examples/nullable-flow) demonstrates a stable snapshot and re-check.

## Get machine-readable evidence

```sh
keika check --json src/main.km
```

Source failures appear in `diagnostics` with original `.km` ranges. Project, lock, or package-load failures appear in the top-level `error`. Do not debug generated Go first when a Kinmokusei diagnostic already exists.

JSON mode writes only the report to standard output, including on an invalid program, and uses exit status `1`. This lets tools parse the report without scraping human text. Lines and columns are one-based; offsets are zero-based UTF-8 bytes. LSP clients translate the same source spans to UTF-16 coordinates for editor ranges.

## Nullable proof disappeared

An assignment, address-taking, mutable capture, aliasing write, unknown call, or control-flow join may invalidate a proof. Bind mutable member storage to an immutable snapshot, then check the snapshot:

```ts
const current = this.user;
if (current === null) { return "guest"; }
return current.name;
```

## Project or lock is stale

```sh
keika deps check
keika target
```

Normal compilation deliberately does not repair dependency state. If you intentionally changed `kinmokusei.toml`, run `keika deps lock`; if a generated dependency file was edited manually, restore it through the dependency command rather than patching `.kinmokusei/deps/`.

## A Go package does not connect

```sh
keika interop audit --json path/to/package
```

Look for `unsupported`, `requires_unsafe`, and package-load failures separately. The package may also depend on a target, build tag, CGO toolchain, generated source, or platform file unavailable for the locked target.

## Build works, run does not

`build` can cross-compile. `run` refuses a non-host locked target. Compare `keika target` with the current host, and use the emitted executable on its intended target.

## Generated Go fails validation

This indicates a compiler invariant or unsupported boundary escaped earlier checking. Reduce to the smallest `.km` input, preserve the original diagnostic and tool versions, and report it. `keika emit-go` is useful for inspection, but generated files are not the source-level fix.

## Editor and CLI disagree

Confirm the VS Code `kinmokusei.server.path` points to the same `keika` binary used in the shell. Restart the language server after upgrading. Both paths share the checker, so persistent differences should include the binary version, project root, target, and minimal source when reported.

See [Diagnostics reference](../reference/diagnostics) for output schema and exit status.
