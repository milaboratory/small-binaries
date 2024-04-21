#!/usr/bin/env bash

set -o errexit
set -o nounset

: "${BUILD_DIR:="build"}"
: "${PACK_DIR:="pack"}"

script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}"

mkdir -p "${PACK_DIR}"

current_version="$(git describe --tags)"

if grep -q -- "-" <<<"${current_version}"; then
    echo "Current version number '${current_version}' has suffix. Did you forget tag the commit?"
    exit 1
fi

ls "${BUILD_DIR}" |
    while read -r os_arch; do

        printf "Packing %s:\n" "${os_arch}"

        bin_path="${BUILD_DIR}/${os_arch}"

        ls "${bin_path}" |
            while read -r bin_name; do

                printf "\t%s %s... " "${bin_name}" "${current_version}"

                mkdir -p "${PACK_DIR}/${bin_name}"

                tar \
                    -C "${bin_path}" \
                    -c -z -f "${PACK_DIR}/${bin_name}/${current_version#v}-${os_arch}.tar.gz" \
                    "${bin_name}"

                printf "\n"

            done
    done
