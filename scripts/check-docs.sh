#!/usr/bin/env bash
set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
temporary_directory=$(mktemp -d)
trap 'rm -rf "$temporary_directory"' EXIT
export GOCACHE="$temporary_directory/go-build-cache"

cd "$repository_root"
go build -trimpath -buildvcs=false -o "$temporary_directory/keika" ./cmd/keika

checked=0
while IFS= read -r source; do
  snippet_path=${source#website/}
  if ! grep -R -F --include='*.md' --exclude-dir=node_modules --exclude-dir=.vitepress -- "$snippet_path" website > /dev/null; then
    printf 'documentation example %s is not embedded by a public page\n' "$source" >&2
    exit 1
  fi
  "$temporary_directory/keika" check "$source"
  expected="${source%.km}.stdout"
  if [[ -f "$expected" ]]; then
    arguments_file="${source%.km}.args"
    if [[ -f "$arguments_file" ]]; then
      mapfile -t program_arguments < "$arguments_file"
      example_binary="$temporary_directory/$(basename "${source%.km}")"
      "$temporary_directory/keika" build -o "$example_binary" "$source"
      actual_output=$("$example_binary" "${program_arguments[@]}")
    else
      actual_output=$("$temporary_directory/keika" run "$source")
    fi
    expected_output=$(<"$expected")
    if [[ "$actual_output" != "$expected_output" ]]; then
      printf 'documentation example %s produced unexpected output\n' "$source" >&2
      printf 'expected: %q\nactual:   %q\n' "$expected_output" "$actual_output" >&2
      exit 1
    fi
  fi
  checked=$((checked + 1))
done < <(find website/snippets -type f -name '*.km' -print | LC_ALL=C sort)

if [[ "$checked" -eq 0 ]]; then
  printf 'no documentation examples were found\n' >&2
  exit 1
fi

while IFS= read -r artifact; do
  source="${artifact%.*}.km"
  if [[ ! -f "$source" ]]; then
    printf 'documentation artifact %s has no matching source %s\n' "$artifact" "$source" >&2
    exit 1
  fi
  if [[ "$artifact" == *.args && ! -f "${artifact%.args}.stdout" ]]; then
    printf 'documentation argument contract %s has no matching output contract\n' "$artifact" >&2
    exit 1
  fi
done < <(find website/snippets -type f \( -name '*.stdout' -o -name '*.args' \) -print | LC_ALL=C sort)

command_line_binary="$temporary_directory/command-line-app"
if [[ ! -x "$command_line_binary" ]]; then
  printf 'command-line documentation example was not built as an executable\n' >&2
  exit 1
fi
if [[ "$("$command_line_binary")" != 'usage: greeter <name> <count>' ]]; then
  printf 'command-line documentation example has an unexpected no-argument response\n' >&2
  exit 1
fi
invalid_number_output=$("$command_line_binary" Kinmokusei many)
if [[ "$invalid_number_output" != *'strconv.Atoi'* || "$invalid_number_output" != *'invalid syntax'* ]]; then
  printf 'command-line documentation example did not report an invalid integer\n' >&2
  exit 1
fi
if [[ "$("$command_line_binary" Kinmokusei 10)" != 'error: count must be from 1 through 5' ]]; then
  printf 'command-line documentation example did not report an out-of-range count\n' >&2
  exit 1
fi

rejected=0
while IFS= read -r source; do
  expected="${source%.km}.stderr"
  if [[ ! -f "$expected" ]]; then
    printf 'invalid documentation example %s has no expected diagnostic fragment\n' "$source" >&2
    exit 1
  fi
  diagnostic_output="$temporary_directory/$(basename "${source%.km}").stderr"
  if "$temporary_directory/keika" check "$source" > /dev/null 2> "$diagnostic_output"; then
    printf 'invalid documentation example %s unexpectedly passed\n' "$source" >&2
    exit 1
  fi
  expected_fragment=$(<"$expected")
  if ! grep -F -- "$expected_fragment" "$diagnostic_output" > /dev/null; then
    printf 'invalid documentation example %s did not produce expected diagnostic fragment %q\n' "$source" "$expected_fragment" >&2
    sed -n '1,12p' "$diagnostic_output" >&2
    exit 1
  fi
  rejected=$((rejected + 1))
done < <(find website/snippets-invalid -type f -name '*.km' -print | LC_ALL=C sort)

while IFS= read -r artifact; do
  source="${artifact%.stderr}.km"
  if [[ ! -f "$source" ]]; then
    printf 'invalid documentation diagnostic %s has no matching source %s\n' "$artifact" "$source" >&2
    exit 1
  fi
done < <(find website/snippets-invalid -type f -name '*.stderr' -print | LC_ALL=C sort)

if [[ "$rejected" -eq 0 ]]; then
  printf 'no invalid documentation examples were found\n' >&2
  exit 1
fi

json_diagnostic="$temporary_directory/diagnostic.json"
json_stderr="$temporary_directory/diagnostic.stderr"
if "$temporary_directory/keika" check --json website/snippets-invalid/null-access.km > "$json_diagnostic" 2> "$json_stderr"; then
  printf 'invalid JSON diagnostic example unexpectedly passed\n' >&2
  exit 1
fi
if [[ -s "$json_stderr" ]]; then
  printf 'JSON diagnostic mode unexpectedly wrote to standard error\n' >&2
  sed -n '1,8p' "$json_stderr" >&2
  exit 1
fi
for fragment in \
  '"valid": false' \
  '"message": "nullable value User | null must be checked against null before member access"' \
  '"path": "website/snippets-invalid/null-access.km"' \
  '"line": 6' \
  '"column": 10'; do
  if ! grep -F -- "$fragment" "$json_diagnostic" > /dev/null; then
    printf 'JSON diagnostic example is missing expected fragment %q\n' "$fragment" >&2
    sed -n '1,24p' "$json_diagnostic" >&2
    exit 1
  fi
done

valid_json="$temporary_directory/valid.json"
"$temporary_directory/keika" check --json website/snippets/hello.km > "$valid_json" 2> "$json_stderr"
if [[ -s "$json_stderr" ]] || ! grep -F -- '"valid": true' "$valid_json" > /dev/null || ! grep -F -- '"diagnostics": []' "$valid_json" > /dev/null; then
  printf 'valid documentation example did not produce the documented JSON envelope\n' >&2
  exit 1
fi

input_json="$temporary_directory/input-error.json"
input_status=0
"$temporary_directory/keika" check --json "$temporary_directory/missing.km" > "$input_json" 2> "$json_stderr" || input_status=$?
if [[ "$input_status" -ne 1 || -s "$json_stderr" ]] || ! grep -F -- '"diagnostics": []' "$input_json" > /dev/null || ! grep -F -- '"valid": false' "$input_json" > /dev/null || ! grep -F -- '"error":' "$input_json" > /dev/null; then
  printf 'missing source did not produce the documented input-error JSON envelope\n' >&2
  exit 1
fi

usage_stdout="$temporary_directory/usage.stdout"
usage_stderr="$temporary_directory/usage.stderr"
for usage_case in unknown-option missing-sources; do
  usage_status=0
  if [[ "$usage_case" == unknown-option ]]; then
    "$temporary_directory/keika" check --json --unknown-option > "$usage_stdout" 2> "$usage_stderr" || usage_status=$?
  else
    (cd "$temporary_directory" && "$temporary_directory/keika" check --json) > "$usage_stdout" 2> "$usage_stderr" || usage_status=$?
  fi
  if [[ "$usage_status" -ne 2 || -s "$usage_stdout" || ! -s "$usage_stderr" ]]; then
    printf 'check --json %s did not produce the documented usage status/streams\n' "$usage_case" >&2
    exit 1
  fi
done

generated="$temporary_directory/hello.go"
"$temporary_directory/keika" emit-go -o "$generated" website/snippets/hello.km
if [[ -n "$(gofmt -d "$generated")" ]]; then
  printf 'documented generated Go is not gofmt-formatted\n' >&2
  exit 1
fi
go build -trimpath -o "$temporary_directory/hello" "$generated"
generated_output=$("$temporary_directory/hello")
expected_generated_output=$(<website/snippets/hello.stdout)
if [[ "$generated_output" != "$expected_generated_output" ]]; then
  printf 'documented generated Go produced unexpected output\n' >&2
  exit 1
fi

generated_package="$temporary_directory/calculator.go"
generated_package_test="$temporary_directory/public_api_test.go"
"$temporary_directory/keika" emit-go \
  -package calculator \
  -o "$generated_package" \
  website/snippets/testing/public_api.km
cp website/snippets/testing/public_api_test.go.txt "$generated_package_test"
go test "$generated_package" "$generated_package_test"

ffi_output="$temporary_directory/incoming-ffi"
"$temporary_directory/keika" ffi generate \
  --manifest website/manifests/scalar-ffi.json \
  -o "$ffi_output"
if [[ -n "$(gofmt -d "$ffi_output/generated_ffi.go")" ]]; then
  printf 'documented incoming C FFI output is not gofmt-formatted\n' >&2
  exit 1
fi
if ! grep -F -- 'func Value() int32' "$ffi_output/generated_ffi.go" > /dev/null; then
  printf 'documented incoming C FFI output does not expose Value() int32\n' >&2
  exit 1
fi

source_package_fixture="website/snippets/external-packages/main.km.txt"
if ! grep -R -F --include='*.md' --exclude-dir=node_modules --exclude-dir=.vitepress -- "${source_package_fixture#website/}" website > /dev/null; then
  printf 'external source-package example is not embedded by a public page\n' >&2
  exit 1
fi
package_example_directory="$temporary_directory/source-packages"
mkdir "$package_example_directory"
package_expected_output=$(<website/snippets/external-packages/main.stdout.txt)
(
  export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
  cd "$package_example_directory"
  "$temporary_directory/keika" new library --module example.com/greeting --license Apache-2.0 greeting
  cp "$repository_root/LICENSE" greeting/LICENSE
  "$temporary_directory/keika" new app demo
  cd demo
  "$temporary_directory/keika" deps add --offline --replace ../greeting example.com/greeting@v0.1.0
  cp "$repository_root/$source_package_fixture" main.km
  cp kinmokusei.toml "$package_example_directory/expected.toml"
  cp kinmokusei.lock "$package_example_directory/expected.lock"
  "$temporary_directory/keika" deps check
  "$temporary_directory/keika" check
  if [[ "$("$temporary_directory/keika" run)" != "$package_expected_output" ]]; then
    printf 'external source-package example produced unexpected output\n' >&2
    exit 1
  fi
  "$temporary_directory/keika" build -o "$package_example_directory/demo-app"
  "$temporary_directory/keika" emit-go -package main -o generated.go
  go build -trimpath -o "$package_example_directory/emitted-app" generated.go
  for example_program in demo-app emitted-app; do
    if [[ "$("$package_example_directory/$example_program")" != "$package_expected_output" ]]; then
      printf 'external source-package %s produced unexpected output\n' "$example_program" >&2
      exit 1
    fi
  done
  # Simulate absent disposable module state without deleting an existing checkout.
  mv .kinmokusei/deps "$package_example_directory/saved-deps"
  "$temporary_directory/keika" deps fetch --offline
  "$temporary_directory/keika" deps check
  "$temporary_directory/keika" check
  cmp kinmokusei.toml "$package_example_directory/expected.toml"
  cmp kinmokusei.lock "$package_example_directory/expected.lock"
)

printf 'verified %d valid examples, %d invalid examples, command arguments/failures, diagnostic JSON and usage errors, generated Go package tests, incoming C FFI, and offline source-package creation/import/build/restoration\n' "$checked" "$rejected"
