#!/usr/bin/env bash
set -euo pipefail

# Prints the highest glibc version required by ELF files under a directory,
# for example "2.28".

# Prevent readelf from translating output strings, like Name that is matched on below.
export LC_ALL=C

if ! command -v readelf >/dev/null; then
  echo "${0##*/}: readelf not found" >&2
  exit 1
fi

readonly DIR="${1:?usage: ${0##*/} <package-dir>}"

version="$(
  find "$DIR" -type f -print0 |
    while IFS= read -r -d '' file; do
      # Scan only files that readelf recognizes as valid ELF files.
      readelf --file-header "$file" >/dev/null 2>&1 || continue
      readelf --version-info "$file" || exit $?
    done |
    sed -n 's/.*Name: GLIBC_\([0-9][0-9.]*\).*/\1/p' |
    sort -V |
    tail -n 1
)"

if [[ -z "$version" ]]; then
  echo "${0##*/}: found no glibc requirement under ${DIR}" >&2
  exit 1
fi

echo "$version"
