#!/bin/sh
# Install root:root 0755 at /usr/local/sbin/pelican-export.
# Paths are from the read-only 2026-09-23 server audit; recheck on source upgrades.
set -eu
[ "$#" -eq 0 ] || exit 64
umask 077
ulimit -t 20
ulimit -v 262144
exec /usr/bin/python3 -I /usr/local/lib/pelican-archive/export.py \
    --database /opt/llm-api-bench/data/benchmarks.db \
    --grader /opt/llm-api-bench/build-balance-alerts-20260922/backend-dist/services/pelicanGrader.js \
    --source-id bench-primary
