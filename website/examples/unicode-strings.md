---
title: Iterate UTF-8 strings
description: Distinguish byte length and indexing from Unicode code-point iteration and byte offsets.
---

# Iterate UTF-8 strings

Kinmokusei `string` guarantees valid UTF-8 in checked source; `bstring` stores
arbitrary immutable bytes. Both use Go `string` storage. Length and indexing
remain byte-oriented; range decodes code points without changing storage.

## Source

<<< ../snippets/unicode-strings.km{ts}

## Run

```sh
keika check unicode-strings.km
keika run unicode-strings.km
```

Expected output:

```text
4 230
0:湯 3:a
```

## Read the output

- `len(text)` is `4`: `湯` occupies three UTF-8 bytes and `a` occupies one.
- `text[0]` has type `byte` and returns the first encoded byte, `230`, rather than a one-character string.
- The two-binding range form yields an `int` byte offset and an `int32` Unicode code point.
- The second code point begins at byte offset `3`, so the offsets are `0` and `3`, not character indexes `0` and `1`.

Use `bstring` indexing/slicing for encoded byte protocols; `string` slicing
panics if a boundary splits a code point. Use range when processing Unicode
code points. Grapheme clusters can still contain multiple code points;
user-perceived text segmentation belongs in an appropriate Go package.

## Conversion and validation

This example demonstrates copied byte/code-point conversions, byte slicing,
invalid UTF-8, embedded NUL and the difference between formatting and encoding:

<<< ../snippets/string-values.km{ts}

Expected output:

<<< ../snippets/string-values.stdout{text}

The first two lines show that byte length is `4`, decoded code-point count is
`2`, and changing a converted slice leaves both original strings unchanged.
The one-byte raw prefix of `湯` is invalid UTF-8; its three-byte text prefix is
valid. `b"\xff\x00"` retains both raw bytes: iteration reports U+FFFD (`65533`) for the
first and NUL (`0`) for the second, at byte offsets `0` and `1`.
The following `true` confirms checked decoding rejects the invalid bytes.

`string(int32(65))` encodes `A`; `Itoa(65)` formats the decimal string `65`. Finally,
the two visually similar accented strings have different bytes and code-point
counts. Neither equality nor iteration normalizes text automatically.

See [Types and values](../book/types-and-values), [Control flow](../book/control-flow#range-loops), and the [type-system reference](../reference/types).
