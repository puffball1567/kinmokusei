# Getting started

This guide takes you from an empty directory to a checked and running `.otm` program. OnsenTamago uses the `ontama` command and the installed Go toolchain.

## Requirements

- A supported 64-bit Linux, macOS, or Windows environment
- Go 1.23 through Go 1.27 when building generated programs

Download the archive for your platform from the [latest release](https://github.com/puffball1567/onsentamago/releases/latest), extract `ontama`, and place it on `PATH`. You can also install the command from source:

```sh
go install github.com/puffball1567/onsentamago/cmd/ontama@v0.1.0
ontama version
```

## Your first program

Save this as `hello.otm`:

<<< ../snippets/hello.otm{ts}

Check it without generating or executing a program:

```sh
ontama check hello.otm
```

Run it through the Go toolchain:

```sh
ontama run hello.otm
```

The output is:

```text
Hello from OnsenTamago
```

Build a native executable or inspect the generated Go:

```sh
ontama build -o hello hello.otm
ontama emit-go hello.otm
```

`emit-go` is not a debug-only escape hatch. Readable generated Go is part of the language contract and may be published as ordinary Go source.

## Next steps

- Learn declarations and control flow in [Language basics](./language-basics).
- Understand value and reference behavior in [Types and data](./types-and-data).
- Connect existing packages in [Go interoperability](./go-interop).
- Set up diagnostics and navigation in [Editor setup](./editor).
