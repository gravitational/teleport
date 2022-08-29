#!/bin/bash
set -eu

main() {
  find . -name go.mod | while read -r f; do
    local d=''
    d="$(dirname "$f")"
    pushd "$d" >/dev/null
    go mod tidy
    if [[ -n "$(git status --porcelain)" ]]; then
      echo -e "Found untidy Go Module at $f." \
        "Please run the following command in your workspace try again:"\
        "\n\tcd $d && go mod tidy" >&2
      exit 1
    fi
    popd >/dev/null
  done
}

main "$@"
