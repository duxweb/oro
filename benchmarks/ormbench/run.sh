#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

COUNT="${COUNT:-3}"
BENTIME="${BENTIME:-1s}"
OUT_DIR="${OUT_DIR:-.bench}"
PATTERN="${PATTERN:-.}"

mkdir -p "$OUT_DIR"

go test -run '^$'
go test -bench="$PATTERN" -benchmem -run '^$' -benchtime="$BENTIME" -count="$COUNT" | tee "$OUT_DIR/bench.txt"

go test -bench='BenchmarkCreate/Oro$' -benchmem -run '^$' -benchtime="$BENTIME" -count=1 -cpuprofile "$OUT_DIR/create.cpu.out" -memprofile "$OUT_DIR/create.mem.out" >/dev/null
go test -bench='BenchmarkWhereList/Oro$' -benchmem -run '^$' -benchtime="$BENTIME" -count=1 -cpuprofile "$OUT_DIR/where_list.cpu.out" -memprofile "$OUT_DIR/where_list.mem.out" >/dev/null
