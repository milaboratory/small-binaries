#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

: "${RELEASE_DIR:="release"}"

script_dir="$(cd "$(dirname "${0}")" && pwd)"
cd "${script_dir}"

ask_yn() {
  local _prompt="${1}"

  while true; do
    read -p "${_prompt} (y/n) [n]: " -r yn

    case "${yn}" in
    "y"|"Y")
      return 0
    ;;
    "n"|"N"|"")
      return 1
    ;;
    *)
      echo "Enter only 'y' or 'n' (case insensitive)"
    ;;
    esac
  done
}

rm -rf "${RELEASE_DIR}"

PACK_DIR="${RELEASE_DIR}/common/";      export PACK_DIR
PACK_RELEASE="true";                    export PACK_RELEASE

./build.sh
./pack.sh

(
    echo "Files to be uploaded:"
    echo ""

    cd release
    find . -type f | sort | sed 's|^|\t|'

    echo ""
)

if ! ask_yn "Do you want to start the upload?"; then
    exit 0
fi

aws s3 sync "${RELEASE_DIR}/" s3://milab-euce1-prod-pkgs-s3-platforma-registry/pub/
