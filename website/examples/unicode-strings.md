---
title: Iterate UTF-8 strings
description: Distinguish byte length and indexing from Unicode code-point iteration and byte offsets.
---

# Iterate UTF-8 strings

Kinmokusei strings use Go's UTF-8 string model. Byte-oriented operations remain byte-oriented; range iteration decodes Unicode code points without changing the original storage.

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

Use indexing and slicing for encoded byte protocols. Use range when processing Unicode code points. Grapheme clusters such as an emoji sequence or a letter plus combining mark can still contain multiple code points; user-perceived text segmentation belongs in an appropriate Go package.

See [Types and values](../book/types-and-values), [Control flow](../book/control-flow#range-loops), and the [type-system reference](../reference/types).
