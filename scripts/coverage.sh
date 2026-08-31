#!/bin/sh
set -eu

minimum=${ONTAMA_MIN_COVERAGE:-87.0}
package_minimum=${ONTAMA_MIN_PACKAGE_COVERAGE:-80.0}
profile=${1:-}
temporary=false
package_report=$(mktemp "${TMPDIR:-/tmp}/ontama-package-coverage.XXXXXX")

if [ -z "$profile" ]; then
  profile=$(mktemp "${TMPDIR:-/tmp}/ontama-coverage.XXXXXX")
  temporary=true
fi

cleanup() {
  rm -f "$package_report"
  if [ "$temporary" = true ]; then
    rm -f "$profile"
  fi
}
trap cleanup EXIT HUP INT TERM

go test -timeout 30m -covermode=atomic -coverpkg=./... -coverprofile="$profile" ./...
awk '
  NR > 1 {
    file = $1
    sub(/:.*/, "", file)
    key = $1 " " $2
    if (!(key in seen)) {
      seen[key] = 1
      files[key] = file
      statements[key] = $2
    }
    if ($3 > 0) hit[key] = 1
  }
  END {
    for (key in seen) {
      package = files[key]
      sub(/\/[^/]+$/, "", package)
      total[package] += statements[key]
      if (hit[key]) covered[package] += statements[key]
    }
    for (package in total) {
      display = package
      sub(/^github[.]com\/puffball1567\/onsentamago\//, "", display)
      percentage = 100 * covered[package] / total[package]
      printf "%s\t%.1f%%\t%.12f\t(%d/%d)\n", display, percentage, percentage, covered[package], total[package]
    }
  }
' "$profile" > "$package_report"
echo "per-package statement coverage:"
awk -F '\t' '{ print $1 "\t" $2 "\t" $4 }' "$package_report" | sort
if ! awk -F '\t' -v minimum="$package_minimum" '
  BEGIN { failed = 0 }
  $3 + 0 < minimum + 0 {
    printf "package %s statement coverage %s is below the required %.1f%%\n", $1, $2, minimum > "/dev/stderr"
    failed = 1
  }
  END { exit failed }
' "$package_report"; then
  exit 1
fi
echo "all package statement coverage meets the required ${package_minimum}%"
summary=$(go tool cover -func="$profile" | awk '/^total:/ { print }')
actual=$(printf '%s\n' "$summary" | sed -n 's/.*[[:space:]]\([0-9][0-9.]*\)%$/\1/p')

if [ -z "$actual" ]; then
  echo "cannot parse total statement coverage" >&2
  exit 1
fi

printf '%s\n' "$summary"
if ! awk -v actual="$actual" -v minimum="$minimum" 'BEGIN { exit !(actual + 0 >= minimum + 0) }'; then
  echo "statement coverage ${actual}% is below the required ${minimum}%" >&2
  exit 1
fi
echo "statement coverage ${actual}% meets the required ${minimum}%"
