#!/bin/sh

xcode_error() {
  # Xcode parses this format and surfaces it as a build error.
  printf '%s:1:1: error: %s\n' "$0" "$*" >&2
}

find_go_executable() {
  # Xcode build phases do not inherit a normal login-shell PATH, so standard
  # Homebrew and Go installer paths are checked explicitly.
  for executable in \
    /opt/homebrew/bin/go \
    /usr/local/bin/go \
    /usr/local/go/bin/go \
    /opt/homebrew/opt/go/libexec/bin/go \
    /usr/local/opt/go/libexec/bin/go
  do
    if [ -x "$executable" ]; then
      printf '%s\n' "$executable"
      return 0
    fi
  done

  return 1
}

find_gomobile_executable() {
  # `go install` writes command binaries to GOBIN, or GOPATH/bin when GOBIN is
  # unset. Check that computed install directory before falling back to PATH.
  if [ -n "${GO_COMMANDS_DIR:-}" ] && [ -x "$GO_COMMANDS_DIR/gomobile" ]; then
    printf '%s\n' "$GO_COMMANDS_DIR/gomobile"
    return 0
  fi

  if command -v gomobile >/dev/null 2>&1; then
    command -v gomobile
    return 0
  fi

  return 1
}
