#!/bin/sh
set -eu

backend_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
workspace=$(mktemp -d)
trap 'rm -rf "$workspace"' EXIT HUP INT TERM

ontama deps check "$backend_dir"
ontama check "$backend_dir/gin/main.otm"
ontama check "$backend_dir/fiber/main.otm"

mkdir -p "$workspace/generatedgin" "$workspace/generatedfiber" "$workspace/contract"
ontama emit-go -package generatedgin -o "$workspace/generatedgin/generated.go" "$backend_dir/gin/main.otm"
ontama emit-go -package generatedfiber -o "$workspace/generatedfiber/generated.go" "$backend_dir/fiber/main.otm"
cp -R "$backend_dir/contract/." "$workspace/contract/"
cp "$backend_dir/.ontama/deps/go.mod" "$backend_dir/.ontama/deps/go.sum" "$workspace/"

(cd "$workspace" && go test -race -tags=ontama_demo_contract ./...)
