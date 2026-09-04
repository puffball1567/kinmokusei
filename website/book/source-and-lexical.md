---
title: Source text and lexical structure
description: Learn exactly how Kinmokusei reads files, whitespace, comments, identifiers, keywords, literals, punctuation, and semicolons.
---

# Source text and lexical structure

Before the compiler can reason about types, it turns UTF-8 source text into tokens. Most formatting is flexible, but literal spelling and statement termination are deliberately small and predictable.

## Source files

Kinmokusei source files conventionally use `.km` and contain UTF-8 text. A file is a module scope, not textual inclusion. Importing a file makes selected declarations available; it does not paste its source into the caller.

```text
application/
├── main.km
└── formatting.km
```

Diagnostics refer to the original path, one-based line and column, and zero-based UTF-8 byte offsets. Generated Go locations are not substituted for source locations.

## Whitespace

Spaces, tabs, and line breaks separate tokens but otherwise carry no meaning. These declarations parse the same way:

```ts
const answer = 42;
```

```ts
const answer =
  42;
```

Line breaks do not insert semicolons. A simple statement still needs its explicit `;` even when the next token starts on a new line.

## Comments

Line comments begin with `//` and continue to the end of the line:

```ts
const timeout = 30; // seconds
```

Block comments begin with `/*` and end at the next `*/`:

```ts
/* The retry count is intentionally conservative.
   Keep this synchronized with the service policy. */
const retries = 3;
```

Block comments are not nested. An unclosed block comment is a lexical diagnostic.

## Identifiers

An identifier begins with `_` or a Unicode letter. Later characters may also be Unicode digits.

```ts
const service2 = "api";
const 温度 = 42;
const _internal = true;
```

Names are case-sensitive: `User`, `user`, and `USER` are different bindings. The compiler separately checks whether public names collide after conversion to generated Go naming conventions.

The single identifier `_` is the blank binding in supported binding lists, ranges, catches, and cases. It means “this position is intentionally unused”; it does not introduce a readable local variable.

## Keywords

Keywords cannot be used as ordinary identifiers. The major families are:

| Family | Keywords |
| --- | --- |
| Declarations | `const`, `let`, `function`, `class`, `struct`, `interface`, `constructor` |
| Relationships | `extends`, `implements`, `virtual`, `override`, `final` |
| Visibility/member kind | `public`, `private`, `protected`, `static` |
| Control flow | `if`, `else`, `while`, `for`, `of`, `switch`, `case`, `default` |
| Branching | `return`, `break`, `continue`, `goto`, `fallthrough` |
| Errors | `try`, `catch`, `finally`, `throw` |
| Concurrency | `go`, `await`, `detach`, `select` |
| Values and operators | `true`, `false`, `nil`, `null`, `new`, `this`, `super`, `as` |
| Boundaries | `import`, `from`, `export`, `defer` |

`type`, `alias`, `distinct`, `enum`, and struct-method `pointer` are currently recognized contextually. Treat them as reserved in those grammar positions even though their lexer representation differs from the fixed keyword set.

## Numeric literals

Integer literals contain decimal digits:

```ts
const count = 42;
const zero = 0;
```

A float literal contains decimal digits and a dot:

```ts
const ratio = 0.25;
const whole = 42.0;
```

A leading sign is a unary operator, not part of the literal:

```ts
const minimum: int8 = -128;
```

Radix prefixes, exponent notation, and numeric separators are not implemented. Untyped integer constants default to `int` without another expected type. A representable expected numeric type may absorb a constant; runtime numeric values never widen implicitly.

## String literals

Strings use double quotes and remain on one source line:

```ts
const plain = "hello";
const escaped = "first\nsecond";
const unicode = "温泉たまご";
```

Escapes follow Go-compatible quoted string escapes because the compiler decodes them with the same contract. A raw newline, missing closing quote, or invalid escape is a lexical diagnostic. Raw-string/backtick and template/interpolation syntax are not implemented.

Strings contain UTF-8 bytes. Indexing returns a byte; range iteration returns a byte offset and Unicode code point.

## Boolean and absence literals

`true` and `false` have type `boolean`. Conditions accept only `boolean`; no value is converted by truthiness.

`null` belongs to checked nullable types such as `User | null`. `nil` is the lower-level Go nil value used at nil-capable Go boundaries. They are intentionally distinct and are not interchangeable by an implicit conversion.

## Punctuation and semicolons

Parentheses group expressions and delimit conditions, parameters, and arguments. Braces delimit blocks and literal bodies. Brackets form arrays, indexing, slicing, and Go-shaped explicit type arguments.

Write a semicolon after:

- imports and binding declarations;
- `return`, `throw`, branch, assignment, update, and expression statements;
- channel sends, raw `go` calls, `defer`, and `detach`;
- grouped C export declarations where the grammar shows one.

Do not write a semicolon after a function, class, struct, interface, loop, switch, select, or `try` block.

```ts
function main(): void {
  let count = 0;
  count++;
  if (count === 1) {
    console("ready");
  }
}
```

## What fails before parsing

The lexer reports invalid UTF-8, an unexpected character or character sequence, an unterminated string/comment, and an invalid string escape. Parsing continues where possible so one check can report more than the first mistake.

With the token boundaries established, [Bindings and scope](./bindings-and-scope) explains how declarations introduce names and how long those names remain valid.
