---
title: Observe control-flow boundaries
description: Verify defer evaluation, explicit switch fallthrough, labeled continue, and a checked forward goto.
---

# Observe control-flow boundaries

This recipe focuses on control transfers whose behavior is easy to misread from syntax alone. Its output records when a deferred argument is captured, whether a switch case is re-tested, and which loop a labeled branch targets.

## Source

<<< ../snippets/control-flow-boundaries.km{ts}

## Run

```sh
keika check control-flow-boundaries.km
keika run control-flow-boundaries.km
```

Expected output:

```text
2 1 one next 3 ready
```

## Read the output

1. `defer fmt.Print(value, " ")` evaluates `value` while it is `1`; changing the binding to `2` does not change the saved argument. The ordinary print runs first, then the deferred call runs at function exit.
2. `fallthrough;` enters the following case body without testing whether its `case 2` expression matches. It must be the last direct statement of a non-final value-switch case.
3. `continue outer;` begins the next iteration of the named outer loop. Only column zero is counted for each of three rows.
4. `goto readyPath;` transfers within one function. The checker allows this forward jump because it enters no nested block, crosses no exception boundary, and skips no declaration.

Prefer structured branches and loops for ordinary code. Labels and `goto` are narrow tools for making an otherwise awkward transfer explicit; the compiler rejects undefined or unused labels and unsafe jump shapes.

Read [Control flow](../book/control-flow) for the semantic model and the [syntax reference](../reference/language#control-flow) for the complete statement forms.
