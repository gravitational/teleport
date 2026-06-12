#!/bin/sh

# Builds the gomobile-generated Enroll XCFramework used by the Verify target.
# This script must run as an Xcode Run Script build phase because it writes the
# generated framework into Xcode's TARGET_TEMP_DIR build directory.

set -eu

log_info() {
  printf 'build-enroll-framework: %s\n' "$*"
}

log_info "Running $0"

if [ -z "${TARGET_TEMP_DIR:-}" ] || [ -z "${SRCROOT:-}" ]; then
  xcode_error "Special Xcode environment variables are not set; this script must be run by Xcode."
  exit 1
fi

. "$SRCROOT/Scripts/helpers.sh"


GO_EXECUTABLE="$(find_go_executable || true)"

if [ -z "$GO_EXECUTABLE" ]; then
  xcode_error "go was not found in known installation paths."
  exit 1
fi

log_info "using go at $GO_EXECUTABLE"

if ! GOBIN="$("$GO_EXECUTABLE" env GOBIN)"; then
  xcode_error "failed to read GOBIN from $GO_EXECUTABLE."
  exit 1
fi

if ! GOPATH="$("$GO_EXECUTABLE" env GOPATH)"; then
  xcode_error "failed to read GOPATH from $GO_EXECUTABLE."
  exit 1
fi

if [ -n "$GOBIN" ]; then
  GO_COMMANDS_DIR="$GOBIN"
else
  GO_COMMANDS_DIR="$GOPATH/bin"
fi

log_info "using tools in $GO_COMMANDS_DIR/"

# gomobile may invoke `go` internally, so put the discovered tool locations on
# PATH before running the bind command.
export PATH="$(dirname "$GO_EXECUTABLE"):$GO_COMMANDS_DIR:$PATH"

GOMOBILE_EXECUTABLE="$(find_gomobile_executable || true)"

if [ -z "$GOMOBILE_EXECUTABLE" ]; then
  xcode_error "gomobile was not found. Install it with: \"$GO_EXECUTABLE\" install golang.org/x/mobile/cmd/gomobile@latest && \"$GO_COMMANDS_DIR/gomobile\" init"
  exit 1
fi


ENROLL_IMPORT_PATH="github.com/gravitational/teleport/lib/mobile/verify/enroll"
ENROLL_XCFRAMEWORK="$TARGET_TEMP_DIR/GeneratedFrameworks/Enroll.xcframework"

log_info "using gomobile at $GOMOBILE_EXECUTABLE"
log_info "building $ENROLL_IMPORT_PATH"
log_info "writing $ENROLL_XCFRAMEWORK"

mkdir -p "$(dirname "$ENROLL_XCFRAMEWORK")"

"$GOMOBILE_EXECUTABLE" bind -target=ios -o "$ENROLL_XCFRAMEWORK" "$ENROLL_IMPORT_PATH"

log_info "finished building Enroll.xcframework"
