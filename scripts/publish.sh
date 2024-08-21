#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

: "${RELEASE_DIR:="release"}"

script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}/.."

publish_package() {
  local _bin_name="${1}"
  local _os="${2}"
  local _arch="${3}"

  pl-pkg publish packages \
    --package-id="${_bin_name}" \
    --skip-existing-packages \
    --os="${_os}" \
    --arch="${_arch}"
  printf "\n"
}

publish_packages() {
  local _bin_name="${1}"

  printf "Publishing '%s'...\n\n" "${_bin_name}"

  publish_package "${_bin_name}" "windows" "x64"

  publish_package "${_bin_name}" "linux" "x64"
  publish_package "${_bin_name}" "linux" "aarch64"

  publish_package "${_bin_name}" "macosx" "x64"
  publish_package "${_bin_name}" "macosx" "aarch64"
}

publish_packages "hello-world"
publish_packages "guided-command"
publish_packages "sleep"
publish_packages "read-file-to-stdout-with-sleep"
