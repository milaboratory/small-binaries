#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}"
package_root="$(pwd)"

(
    echo "#"
    echo "# Functional tests: line-counter"
    echo "#"
    go test ./...
    echo ""
)

(
    echo "#"
    echo "# Integration tests: line-counter"
    echo "#"
    echo ""
    echo "# Building line-counter"
    cd cmd/line-counter
    go build .
    echo ""

    cd "${package_root}"
    mkdir -p tests
    cd tests

    counter="${package_root}/cmd/line-counter/line-counter"

    echo "# Test case: plain file"
    printf 'a\nb\nc\n' > "./plain.txt"
    "${counter}" --input "./plain.txt" --output "./plain.count"
    if [ "$(cat ./plain.count)" != "3" ]; then
        echo "expected 3, got '$(cat ./plain.count)'"
        exit 1
    fi
    echo "OK"
    echo ""

    echo "# Test case: gzip file"
    printf 'x\ny\n' | gzip > "./data.txt.gz"
    "${counter}" --input "./data.txt.gz" --output "./gz.count"
    if [ "$(cat ./gz.count)" != "2" ]; then
        echo "expected 2, got '$(cat ./gz.count)'"
        exit 1
    fi
    echo "OK"
    echo ""

    echo "# Test case: no trailing newline in output"
    if [ "$(wc -c < ./plain.count | tr -d ' ')" != "1" ]; then
        echo "output must contain exactly the integer with no trailing newline"
        exit 1
    fi
    echo "OK"
    echo ""

    echo "# Test case: missing input"
    if "${counter}" --input "./does-not-exist.txt" --output "./missing.count" 2>/dev/null; then
        echo "line-counter run on missing file must return non-zero exit code"
        exit 1
    fi
    echo "OK"
    echo ""
)
