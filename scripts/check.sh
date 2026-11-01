#!/usr/bin/env bash
# Full local gate: format, vet, lint, tests (including the architecture test).
set -euo pipefail
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "gofmt needed on:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go vet ./...
golangci-lint run ./...
go test ./...
