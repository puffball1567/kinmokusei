# Installing OnsenTamago

OnsenTamago is distributed as the `ontama` command. A normal installation uses
a released binary; users do not need a checkout of the compiler source.
Go 1.23 through Go 1.27 are supported because `build`, `run`, dependency
resolution, and direct Go package interop use the Go toolchain. The release
archives contain a compiler built with Go 1.27, which can consume package export
data from every supported version.

The compiler reads package export data using the Go version with which it was
built. If installing `ontama` from source, build it with the newest Go version
you intend to target. Support for a later Go minor release may require a
matching OnsenTamago release.

## Release archives

Download the archive for the current platform from
[GitHub Releases](https://github.com/puffball1567/onsentamago/releases). Each
release provides these assets:

| Platform | Archive |
| --- | --- |
| Linux x86-64 | `ontama_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `ontama_<version>_linux_arm64.tar.gz` |
| macOS Intel | `ontama_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `ontama_<version>_darwin_arm64.tar.gz` |
| Windows x86-64 | `ontama_<version>_windows_amd64.zip` |

Compare the downloaded file with `SHA256SUMS` before installation. Linux can
use `sha256sum -c SHA256SUMS`; macOS can use `shasum -a 256 -c SHA256SUMS`.
Windows PowerShell provides `Get-FileHash <archive> -Algorithm SHA256` for
comparison with the published value.

Extract the archive and place `ontama` (`ontama.exe` on Windows) in a directory
on `PATH`. A per-user Unix installation can use `~/.local/bin`; a system-wide
installation commonly uses `/usr/local/bin`. On Windows, keep `ontama.exe` in a
dedicated tools directory and add that directory to the user `Path` setting.

For example, a per-user Linux x86-64 installation of v0.1.0 is:

```sh
tar -xzf ontama_0.1.0_linux_amd64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 ontama_0.1.0_linux_amd64/ontama ~/.local/bin/ontama
```

On macOS, use the matching `darwin` archive with the same commands. On Windows,
use **Expand-Archive** or File Explorer to extract the ZIP, then move
`ontama.exe` into the tools directory selected for `Path`.

Confirm both tools:

```sh
ontama version
go version
```

## Install with the Go toolchain

Developers with Go already installed may install a tagged compiler directly:

```sh
go install github.com/puffball1567/onsentamago/cmd/ontama@v0.1.0
```

Go writes the executable to `GOBIN`, or to the Go bin directory when `GOBIN`
is unset. That directory must be on `PATH`.

## Build from a source checkout

From the repository root:

```sh
go build -trimpath -o ./ontama ./cmd/ontama
./ontama version
```

An untagged source build reports `devel`. It has the same compiler behavior as
the corresponding source revision.

## Write and run a program

Create `hello.otm` (the same source is kept in
[`examples/hello/main.otm`](../examples/hello/main.otm)):

```ts
import go fmt from "fmt";

function main(): void {
  fmt.Println("Hello from OnsenTamago");
}
```

Then check and run it:

```sh
ontama check hello.otm
ontama run hello.otm
```

`check` performs parsing and semantic checking without generating or executing
the program. `run` emits Go into the project-local `.ontama` state directory,
builds it with the selected Go toolchain, and executes it. Use `emit-go` when the
generated Go source itself is the desired artifact.

## Visual Studio Code

The matching `onsentamago-<version>.vsix` is attached to the same GitHub
release. Install it from Visual Studio Code with **Extensions: Install from
VSIX**, then ensure `ontama` is on the editor process's `PATH`. Alternatively,
set `onsentamago.server.path` to the executable. The extension and compiler
should use the same release version.
