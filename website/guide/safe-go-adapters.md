---
title: Safe generic Go adapters
description: Observe, format, snapshot and extract errors from generic values without erasing native storage contracts.
---

# Safe generic Go adapters

::: info Development APIs
The observations, snapshot bridge and direct `fmt.Sprintf` contract on this page
are development changes after v0.4.9, not APIs available in that release.
:::

Generic assertion and diagnostic libraries need to work with user records, not
only a finite union of scalar types. The safe boundary distinguishes observing
a value from returning a capability that can modify it.

<<< ../snippets/safe-go-adapters.km{ts}

## Observe without exposing reflection

```ts
function isNil<T>(value: T): boolean {
  return inspectNil(value);
}

function same<A, B>(actual: A, expected: B): boolean {
  if (!inspectSameType(actual, expected)) { return false; }
  if (!inspectComparable(actual) || !inspectComparable(expected)) { return false; }
  return inspectEqual(actual, expected);
}
```

`inspectNil` handles typed nils, not just a nil interface. `inspectEqual` is
identity/value equality, not `DeepEqual`: different pointers to equal objects
remain different. NaN remains unequal to itself and positive/negative zero
compare equal, as in Go. A same-value matcher can additionally use
`inspectFloat` with `math.IsNaN` and `math.Signbit` for its chosen semantics.
`inspectString` reads raw string contents without invoking user formatting
methods. See the [complete operation table](../reference/built-ins#safe-observations-development).

These functions never return a reflective view. Do not replace them with an
unrestricted `reflect.ValueOf` wrapper; such a wrapper would expose writable
aliases. Runtime Go type observations do not reconstruct source UTF-8 proofs.

## Keep formatting deferred

Direct `fmt.Sprintf` calls have a separately audited callback-aware contract:

```ts
import go fmt from "fmt";

function render<T>(value: T): bstring {
  return fmt.Sprintf("%#v", value);
}

class Argument implements fmt.GoStringer {
  constructor(private render: () => bstring) {}
  public function GoString(): bstring { return this.render(); }
}

function message<A, B>(expected: A, actual: B): bstring {
  const left = new Argument((): bstring => render(expected));
  const right = new Argument((): bstring => render(actual));
  return fmt.Sprintf("expected %#v, got %#v", left, right);
}
```

The source arguments are evaluated before the call. The zero-argument closures
capture those values without eagerly rendering them. A formatter on `expected`
can mutate application state before `actual` is rendered, but an already
captured scalar `actual` keeps its original value. Pointer/container captures
retain ordinary shared-reference semantics; this is not a deep snapshot.

Sprintf may invoke `Format`, `GoString`, `String` and `Error`; those methods may
mutate checked state. Go controls their invocation and panic formatting. There
is no extra purity, traversal-depth, cycle-bounding or synchronization guarantee.
Frameworks retain responsibility for their own bounded diagnostic traversal.
The generated call returns only raw `bstring`, never an argument alias.

Keep the Go call direct inside the wrapper. Namespace/named import aliases are
recognized by resolved function identity, but an unknown stored function value
does not inherit the contract. `fmt.Errorf` is not covered: `%w` retains the
wrapped error, unlike a text-only result. Nor is there a blanket exemption for
writers, reflection or other opaque functions.

## Snapshot generic values

```ts
function snapshot<T>(value: T): Result<byte[]> {
  return jsonSnapshot(value, "", "  ");
}
```

Unlike a direct generic `json.MarshalIndent` call, the compiler-owned bridge
guarantees a byte result or a **text-only error**. The original error may itself
expose the input via reflection, so its concrete type, identity and unwrap chain
are deliberately not returned. If error-type matching matters, retain an
ordinary Go call on a separately constructed raw DTO instead. JSON decoding
still requires raw DTO fields followed by explicit validation.

Use the [checked input adapter](./input-adapters) to share the read,
construction and publication pipeline across generic framework binders.

Encoding may invoke custom methods; it is not a pure operation. Prefix/indent,
bytes, error and callback behavior are specified in the
[snapshot reference](../reference/built-ins#json-snapshots-development).

## Supply a concrete error extraction adapter

A generic library must not send an unconstrained writable `*T` to `errors.As`
or assert `*T` out of an arbitrary interface. Neither operation proves native
UTF-8, class initialization or nullability contracts. Instead, accept typed
allocation/extraction callbacks implemented where the concrete raw Go target
is known:

```ts
import go errors from "errors";
import go fs from "io/fs";

function extract<T>(
  source: error,
  allocate: () => *T,
  apply: (source: error, target: *T) => boolean,
): (T, boolean) {
  const target = allocate();
  const matched = apply(source, target);
  return *target, matched;
}

function extractPathError(source: error): (*fs.PathError, boolean) {
  return extract<*fs.PathError>(
    source,
    () => { let value: *fs.PathError = nil; return &value; },
    (err, target) => errors.As(err, target),
  );
}
```

Go automatically manages the escaping local's lifetime; no manual allocation or
free is needed. The compiler checks the concrete `**fs.PathError` call as usual.
The generic library can share this adapter across error matchers without knowing
the user's Go type. An API previously assuming arbitrary reflective construction
needs explicit adapter/factory parameters; there is no new blanket raw-storage
constraint or unchecked generic assertion.

Always read the target **even when the match is false**. A custom `As` method can
write it and then decline the match. This design retains that mutation, pointer
identity, wrapped/joined error traversal, and typed nil targets. Do not replace
it with validation only on success or an eagerly discarded temporary. Native
verified-text destinations still need a checked domain-specific conversion;
the raw extraction callback cannot bypass those checks.
