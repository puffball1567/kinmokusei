---
title: Encode JSON with a Go package
description: Pass a structural Kinmokusei object directly to encoding/json and propagate its error.
---

# Encode JSON with a Go package

This recipe uses a structural object as a data-transfer value and calls the real Go `encoding/json` package.

## Project tree

```text
json-value/
└── main.km
```

## Source

<<< ../snippets/json.km{ts}

## Run

```sh
keika check main.km
keika run main.km
```

Expected output:

```text
{"guest":"Aki","temperature":42} true
```

## Contract demonstrated

- The structural object type supplies an exact expected shape for the literal.
- Generated anonymous Go fields are public and receive deterministic JSON tags preserving `guest` and `temperature`.
- `json.Marshal` is resolved from real Go export/type data.
- Its `([]byte, error)` result crosses into `Result<string>` only through explicit postfix `?`.
- `string(encoded)` is a Go-compatible byte-slice conversion.

For a complete server using direct `net/http` and `encoding/json`, see the repository's [JSON API source](https://github.com/puffball1567/kinmokusei/tree/main/examples/json-api).
