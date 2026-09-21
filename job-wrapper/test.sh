#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}"

(
    echo "#"
    echo "# Unit tests: job-wrapper"
    echo "#"
    go vet ./...
    go test ./internal/...
    echo ""
)

(
    echo "#"
    echo "# Integration tests on the host: legacy job-script.sh contract, report, signals"
    echo "#"
    go test ./tests/ -run 'Test[^D]' -count=1
    echo ""
)

(
    echo "#"
    echo "# Integration tests in Docker: cgroup metrics, OOM kill, online report, PID 1 duties"
    echo "#"
    if [ -n "${SKIP_DOCKER_TESTS:-}" ]; then
        echo "SKIP_DOCKER_TESTS is set: skipping"
        exit 0
    fi
    if ! docker info >/dev/null 2>&1; then
        echo "docker is not available. The Docker suite is mandatory; set SKIP_DOCKER_TESTS=1 to skip it on purpose." >&2
        exit 1
    fi
    JOB_WRAPPER_DOCKER=1 go test ./tests/ -run 'TestDocker' -count=1 -timeout 15m
    echo ""
)
