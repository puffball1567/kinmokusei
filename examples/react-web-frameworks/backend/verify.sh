#!/bin/sh
set -eu

backend_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM

keika deps check "$backend_dir"
keika check "$backend_dir/gin/main.km"
keika check "$backend_dir/fiber/main.km"

mkdir -p "$workspace/generatedgin" "$workspace/generatedfiber" "$workspace/contract"
keika emit-go -package generatedgin -o "$workspace/generatedgin/generated.go" "$backend_dir/gin/main.km"
keika emit-go -package generatedfiber -o "$workspace/generatedfiber/generated.go" "$backend_dir/fiber/main.km"
cp -R "$backend_dir/contract/." "$workspace/contract/"
cp "$backend_dir/.kinmokusei/deps/go.mod" "$backend_dir/.kinmokusei/deps/go.sum" "$workspace/"

(cd "$workspace" && go test -race -tags=kinmokusei_demo_contract ./...)
