#!/usr/bin/env bash
set -euo pipefail

go version

go mod download

go build ./...
go vet ./...

gofmt_bin="$(go env GOROOT)/bin/gofmt"
unformatted="$(${gofmt_bin} -l .)"
if [[ -n "${unformatted}" ]]; then
  echo "Go files need formatting:" >&2
  echo "${unformatted}" >&2
  exit 1
fi

go test ./...

go mod tidy
git diff --exit-code go.mod go.sum
