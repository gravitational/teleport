#!/bin/bash
#
# Builds libfido2 and dependencies, caching the resulting binaries in the local
# filesystem.
#
# Run `build-fido2-macos.sh build` to build libfido2 and its dependencies, at
# the versions specified in the script.
# Run `build-fido2-macos.sh pkg_config_path` to print the path to the
# prior-built libfido2-static.pc file.
#
# Written mainly for macOS builders.
set -eu

# Note: versions are the same as the corresponding git tags for each repo.
readonly CBOR_VERSION=v0.9.0
readonly CRYPTO_VERSION=OpenSSL_1_1_1o
readonly FIDO2_VERSION=1.11.0

readonly LIB_CACHE="/tmp/teleport-fido2-cache"

readonly PKGFILE_DIR="$LIB_CACHE/fido2-${FIDO2_VERSION}_cbor-${CBOR_VERSION}_crypto-${CRYPTO_VERSION}"

fetch_and_build() {
  local name="$1"      # eg, cbor
  local version="$2"   # eg, v0.9.0
  local url="$3"       # eg, https://github.com/...
  local buildcmd="$4"  # eg, cbor_build, a bash function name
  echo "$name: fetch and build" >&2

  local tmp=''
  tmp="$(mktemp -d "$LIB_CACHE/build.XXXXXX")"
  # Early expansion on purpose.
  #shellcheck disable=SC2064
  trap "rm -fr '$tmp'" exit

  local fullname="$name-$version"
  local install_path="$tmp/$fullname"

  cd "$tmp"
  git clone --depth=1 -b "$version" "$url"
  cd "$(ls)"  # a single folder exists at this point
  mkdir -p "$install_path"
  eval "$buildcmd '$PWD' '$install_path'"

  # Fix path in pkgconfig files.
  local dest="$LIB_CACHE/$fullname"
  find "$install_path" \
    -name '*.pc' \
    -exec sed -i '' "s@$install_path@$dest@g" {} +

  # Check if another builder beat us. Builds _should_ be equivalent.
  mkdir -p "$LIB_CACHE"
  if [[ ! -d "$dest" ]]; then
    echo "$name: moving $fullname to $dest" >&2
    mv "$install_path" "$dest"
  fi
}

cbor_build() {
  local src="$1"
  local dest="$2"
  echo 'cbor: building' >&2
  cd "$src"

  cmake \
    -DCBOR_CUSTOM_ALLOC=ON \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$dest" \
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
    -DWITH_EXAMPLES=OFF \
    -G "Unix Makefiles" \
    .
  make
  make install
}

cbor_fetch_and_build() {
  fetch_and_build \
    cbor "$CBOR_VERSION" 'https://github.com/pjk/libcbor.git' cbor_build
}

crypto_build() {
  local src="$1"
  local dest="$2"
  echo 'crypto: building' >&2
  cd "$src"

  ./config \
    -mmacosx-version-min=10.12 \
    --prefix="$dest" \
    --openssldir="$dest/openssl@1.1" \
    no-shared \
    no-zlib
  # Build and copy only what we need instead of 'make && make install'.
  # It's a bit quicker.
  make build_generated libcrypto.a libcrypto.pc
  mkdir -p "$dest/"{include,lib/pkgconfig}
  cp -r include/openssl "$dest/include/"
  cp libcrypto.a "$dest/lib/"
  cp libcrypto.pc "$dest/lib/pkgconfig"
}

crypto_fetch_and_build() {
  fetch_and_build \
    crypto "$CRYPTO_VERSION" 'https://github.com/openssl/openssl.git' \
    crypto_build
}

fido2_build() {
  local src="$1"
  local dest="$2"
  echo 'fido2: building' >&2
  cd "$src"

  export PKG_CONFIG_PATH="$LIB_CACHE/cbor-$CBOR_VERSION/lib/pkgconfig:$LIB_CACHE/crypto-$CRYPTO_VERSION/lib/pkgconfig"
  cmake \
    -DBUILD_EXAMPLES=OFF \
    -DBUILD_MANPAGES=OFF \
    -DBUILD_TOOLS=OFF \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$dest" \
    -G "Unix Makefiles" \
    .
  make
  make install
}

fido2_fetch_and_build() {
  fetch_and_build \
    fido2 "$FIDO2_VERSION" 'https://github.com/Yubico/libfido2.git' fido2_build
}

usage() {
  echo "Usage: $0 build|pkg_config_path" >&2
}

build() {
  local cbor_path="$LIB_CACHE/cbor-$CBOR_VERSION"
  local crypto_path="$LIB_CACHE/crypto-$CRYPTO_VERSION"
  local fido2_path="$LIB_CACHE/fido2-$FIDO2_VERSION"

  if [[ ! -d "$cbor_path" ]]; then
    cbor_fetch_and_build
  fi

  if [[ ! -d "$crypto_path" ]]; then
    crypto_fetch_and_build
  fi

  if [[ ! -d "$fido2_path" ]]; then
    fido2_fetch_and_build
  fi

  local pkgfile="$PKGFILE_DIR/libfido2-static.pc"
  if [[ ! -f "$pkgfile" ]]; then
    local tmp=''
    tmp="$(mktemp)"  # file, not dir!
    # Early expansion on purpose.
    #shellcheck disable=SC2064
    trap "rm -f '$tmp'" EXIT

    # Write libfido2-static.pc to tmp.
    local cbor="$LIB_CACHE/cbor-$CBOR_VERSION"
    local crypto="$LIB_CACHE/crypto-$CRYPTO_VERSION"
    local fido2="$LIB_CACHE/fido2-$FIDO2_VERSION"
    cat >"$tmp" <<EOF
prefix=$fido2
exec_prefix=\${prefix}
libdir=\${prefix}/lib
includedir=\${prefix}/include

Name: libfido2
Description: A FIDO2 library
URL: https://github.com/yubico/libfido2
Version: $FIDO2_VERSION
Libs: -framework CoreFoundation -framework IOKit \${libdir}/libfido2.a $cbor/lib/libcbor.a $crypto/lib/libcrypto.a
Cflags: -I\${includedir} -I$cbor/include -I$crypto/include
EOF

    # Move .pc file to expected path.
    mkdir -p "$PKGFILE_DIR"
    if [[ ! -f "$pkgfile" ]]; then
      echo "fido2: creating $pkgfile" >&2
      mv "$tmp" "$pkgfile"
    fi
  fi
}

main() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 1
  fi

  case "$1" in
    build)
      build
      ;;
    pkg_config_path)
      echo "$PKGFILE_DIR"
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
