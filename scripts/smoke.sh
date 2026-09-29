#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

echo '[1/7] gofmt'
gofmt -w cmd internal

echo '[2/7] go test'
go test ./...

echo '[3/7] go test -race'
go test -race ./...

echo '[4/7] go vet'
go vet ./...

echo '[5/7] build Go engine'
go build -o /tmp/kol-actor-engine ./cmd/kol-actor-engine

echo '[6/7] compile available language clients'
gcc -Wall -Wextra -Werror clients/c/client.c -o /tmp/kol-actor-client-c
g++ -std=c++20 -Wall -Wextra -Werror clients/cpp/main.cpp -o /tmp/kol-actor-client-cpp
kotlinc clients/kotlin/Main.kt -include-runtime -d /tmp/kol-actor-client-kotlin.jar
swiftc clients/swift/main.swift -o /tmp/kol-actor-client-swift
if command -v rustc >/dev/null 2>&1; then
  rustc --edition=2021 clients/rust/src/main.rs -o /tmp/kol-actor-client-rust
else
  echo 'rustc unavailable: Rust source present but not compiled'
fi
if command -v dotnet >/dev/null 2>&1; then
  echo 'dotnet available; C# source is a single-file client and can be placed in a console project'
else
  echo 'dotnet unavailable: C# source present but not compiled'
fi

echo '[7/7] ASH static guard checks'
grep -q 'ASH execution authority: false' kolmafia/scripts/kol_actor.ash
if grep -Eq 'cli_execute\(|visit_url\([^,]+,[[:space:]]*true' kolmafia/scripts/kol_actor.ash; then
  echo 'unexpected mutating ASH primitive found' >&2
  exit 1
fi

echo 'SMOKE PASS'
