#!/usr/bin/env bash
set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
temporary_directory=$(mktemp -d)
trap 'rm -rf "$temporary_directory"' EXIT

cd "$repository_root"
go build -trimpath -o "$temporary_directory/ontama" ./cmd/ontama

checked=0
while IFS= read -r source; do
  "$temporary_directory/ontama" check "$source"
  expected="${source%.otm}.stdout"
  if [[ -f "$expected" ]]; then
    actual_output=$("$temporary_directory/ontama" run "$source")
    expected_output=$(<"$expected")
    if [[ "$actual_output" != "$expected_output" ]]; then
      printf 'documentation example %s produced unexpected output\n' "$source" >&2
      printf 'expected: %q\nactual:   %q\n' "$expected_output" "$actual_output" >&2
      exit 1
    fi
  fi
  checked=$((checked + 1))
done < <(find website/snippets -type f -name '*.otm' -print | LC_ALL=C sort)

if [[ "$checked" -eq 0 ]]; then
  printf 'no documentation examples were found\n' >&2
  exit 1
fi

printf 'verified %d documentation examples\n' "$checked"
