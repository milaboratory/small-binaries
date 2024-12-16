#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail


script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}"
package_root="$(pwd)"

(
    echo "#"
    echo "# Functional tests: table-converter"
    echo "#"
    go test ./...
    echo ""
)

(
    echo "#"
    echo "# Integration tests: table-converter"
    echo "#"
    echo ""
    echo "# Building table-converter"
    cd cmd/table-converter
    go build .
    echo ""

    cd "${package_root}"
    mkdir -p tests
    cd tests

    converter="${package_root}/cmd/table-converter/table-converter"
    integrations="${package_root}/integrations"

    echo "# Test case: canonical"
    "${converter}" "${integrations}/canonical_in.scsv" "./canonical_out.scsv"
    diff "${integrations}/canonical_out.scsv" "./canonical_out.scsv"
    echo "OK"
    echo ""

    echo "# Test case: select metrics"
    "${converter}" -metric-columns-search '^M1$' "${integrations}/canonical_in.scsv" "./canonical_out.scsv"
    diff "${integrations}/canonical_out_selection.scsv" "./canonical_out.scsv"
    echo "OK"
    echo ""

    echo "# Test case: custom (sample by name)"
    "${converter}" -sample-column-name M -separator ' ' "${integrations}/custom_in.csv" "./custom_out.csv"
    diff "${integrations}/custom_out.csv" "./custom_out.csv"
    echo "OK"
    echo ""

    echo "# Test case: custom (sample by index)"
    "${converter}" -sample-column-i 1 -separator ' ' "${integrations}/custom_in.csv" "./custom_out.csv"
    diff "${integrations}/custom_out.csv" "./custom_out.csv"
    echo "OK"
    echo ""

    echo "# Test case: custom labels"
    "${converter}" \
        -sample-column-name M \
        -separator ' ' \
        --sample-label S \
        --metric-label M \
        --value-label V \
        "${integrations}/custom_in.csv" "./custom_out_labels.csv"
    diff "${integrations}/custom_out_labels.csv" "./custom_out_labels.csv"
    echo "OK"
    echo ""

    echo "# Test case: custom (sample by RE)"
    "${converter}" -sample-column-search '^M$' -separator ' ' "${integrations}/custom_in.csv" "./custom_out.csv"
    diff "${integrations}/custom_out.csv" "./custom_out.csv"
    echo "OK"
    echo ""

    echo "# Test case: malformed file"
    if "${converter}" "${integrations}/malformed.csv" "./malformed_out.csv" 2>/dev/null; then
        echo "converter run on malformed file must return non-zero exit code"
        exit 1
    fi
    echo "OK"
    echo ""
)
