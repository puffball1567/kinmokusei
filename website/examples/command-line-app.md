---
title: Build a command-line application
description: Read positional arguments, validate input through Result, and build a native executable with deterministic behavior.
---

# Build a command-line application

This recipe builds a small native command that reads `os.Args`, validates a bounded repeat count, and prints a greeting. It uses ordinary Go process arguments, so there is no Kinmokusei-specific argument runtime.

## Source

<<< ../snippets/command-line-app.km{ts}

## Build and run

::: code-group

```sh [Linux and macOS]
keika check main.km
keika build -o ./greeter main.km
./greeter Kinmokusei 3
```

```powershell [Windows PowerShell]
keika check .\main.km
keika build -o .\greeter.exe .\main.km
.\greeter.exe Kinmokusei 3
```

:::

Expected output:

```text
Hello, Kinmokusei!
Hello, Kinmokusei!
Hello, Kinmokusei!
```

`keika run` currently treats all positional arguments as source inputs; it does not forward arguments to the generated program. Build an executable when testing `os.Args`, as shown above.

## Separate parsing from effects

`main` owns the process boundary, while `parseCount` and `render` remain ordinary functions:

- `strconv.Atoi(text)?` converts a Go `(int, error)` directly into the `Result<int>` early-return path.
- the range check turns a syntactically valid but unacceptable count into an application error;
- `render` can be tested without changing process-global arguments;
- `makeSlice` reserves capacity, and every `append` result is assigned back because growth may replace the backing array.

`os.Args[0]` is the executable path. User arguments begin at index 1, so this command requires exactly three entries: executable, name, and count. The length check happens before either index operation.

## Failure behavior

These calls exercise distinct user errors:

```sh
./greeter
./greeter Kinmokusei many
./greeter Kinmokusei 10
```

They print a usage line, a `strconv.Atoi` error, and the application range error respectively. This example reports expected input failures without panicking; a larger command can map them to explicit exit codes in a neighboring Go entry package when required.

The documentation test builds this source, passes `Kinmokusei` and `3` as separate process arguments, and compares all three output lines exactly. See the [CLI reference](../reference/cli) for compiler-command behavior and [testing applications](../guide/testing) for executable and generated-package test strategies.
