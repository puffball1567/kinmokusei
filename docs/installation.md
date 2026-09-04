# Installing Kinmokusei

Kinmokusei is distributed as the `keika` command. A normal installation uses
a released binary; users do not need a checkout of the compiler source.
Go 1.23 through Go 1.27 are supported because `build`, `run`, dependency
resolution, and direct Go package interop use the Go toolchain. The release
archives contain a compiler built with Go 1.27, which can consume package export
data from every supported version.

The compiler reads package export data using the Go version with which it was
built. If installing `keika` from source, build it with the newest Go version
you intend to target. Support for a later Go minor release may require a
matching Kinmokusei release.

## Release archives

Download the archive for the current platform from
[GitHub Releases](https://github.com/puffball1567/kinmokusei/releases). Each
Kinmokusei-named release provides these assets. Users upgrading from v0.1 should
follow the [v0.1 migration guide](migrating-from-v0.1.md).

| Platform | Archive |
| --- | --- |
| Linux x86-64 | `keika_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `keika_<version>_linux_arm64.tar.gz` |
| macOS Intel | `keika_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `keika_<version>_darwin_arm64.tar.gz` |
| Windows x86-64 | `keika_<version>_windows_amd64.zip` |

Compare the downloaded file with `SHA256SUMS` before installation. Linux can
use `sha256sum -c SHA256SUMS`; macOS can use `shasum -a 256 -c SHA256SUMS`.
Windows PowerShell provides `Get-FileHash <archive> -Algorithm SHA256` for
comparison with the published value.

Extract the archive and place `keika` (`keika.exe` on Windows) in a directory
on `PATH`. A per-user Unix installation can use `~/.local/bin`; a system-wide
installation commonly uses `/usr/local/bin`. On Windows, keep `keika.exe` in a
dedicated tools directory and add that directory to the user `Path` setting.

For example, a per-user Linux x86-64 installation of version `X.Y.Z` is:

```sh
tar -xzf keika_X.Y.Z_linux_amd64.tar.gz
mkdir -p ~/.local/bin
install -m 0755 keika_X.Y.Z_linux_amd64/keika ~/.local/bin/keika
```

On macOS, use the matching `darwin` archive with the same commands. On Windows,
use **Expand-Archive** or File Explorer to extract the ZIP, then move
`keika.exe` into the tools directory selected for `Path`.

Confirm both tools:

```sh
keika version
go version
```

## Install with the Go toolchain

Developers with Go already installed may install the latest tagged compiler
directly:

```sh
go install github.com/puffball1567/kinmokusei/cmd/keika@latest
```

Go writes the executable to `GOBIN`, or to the Go bin directory when `GOBIN`
is unset. That directory must be on `PATH`.

## Build from a source checkout

From the repository root:

```sh
go build -trimpath -o ./keika ./cmd/keika
./keika version
```

An untagged source build reports `devel`. It has the same compiler behavior as
the corresponding source revision.

## Write and run a program

Create `hello.km` (the same source is kept in
[`examples/hello/main.km`](../examples/hello/main.km)):

```ts
import go fmt from "fmt";

function main(): void {
  fmt.Println("Hello from Kinmokusei");
}
```

Then check and run it:

```sh
keika check hello.km
keika run hello.km
```

`check` performs parsing and semantic checking without generating or executing
the program. `run` emits Go into the project-local `.kinmokusei` state directory,
builds it with the selected Go toolchain, and executes it. Use `emit-go` when the
generated Go source itself is the desired artifact.

## Visual Studio Code

The matching `kinmokusei-<version>.vsix` is attached to the same GitHub
release. Install it from Visual Studio Code with **Extensions: Install from
VSIX**, then ensure `keika` is on the editor process's `PATH`. Alternatively,
set `kinmokusei.server.path` to the executable. The extension and compiler
should use the same release version.
