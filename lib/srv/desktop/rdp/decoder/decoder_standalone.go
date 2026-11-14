//go:build !desktop_access_rdp && rust_rdp_decoder

// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package decoder

/*
// Link the completed static archive directly so cgo does not scan Cargo's intermediate objects.
#cgo linux,386 LDFLAGS: ${SRCDIR}/../../../../../target/i686-unknown-linux-gnu/release/librdp_decoder.a
#cgo linux,amd64 LDFLAGS: ${SRCDIR}/../../../../../target/x86_64-unknown-linux-gnu/release/librdp_decoder.a
#cgo linux,arm LDFLAGS: ${SRCDIR}/../../../../../target/arm-unknown-linux-gnueabihf/release/librdp_decoder.a
#cgo linux,arm64 LDFLAGS: ${SRCDIR}/../../../../../target/aarch64-unknown-linux-gnu/release/librdp_decoder.a
// The system libraries below are based on:
//   cargo rustc --release --target x86_64-unknown-linux-gnu -p rdp-decoder \
//     --crate-type staticlib -- --print native-static-libs
#cgo linux LDFLAGS: -static-libgcc -lutil -lrt -lpthread -lm -ldl -lc

#cgo darwin,amd64 LDFLAGS: ${SRCDIR}/../../../../../target/x86_64-apple-darwin/release/librdp_decoder.a
#cgo darwin,arm64 LDFLAGS: ${SRCDIR}/../../../../../target/aarch64-apple-darwin/release/librdp_decoder.a

#cgo windows,amd64 LDFLAGS: ${SRCDIR}/../../../../../target/x86_64-pc-windows-gnu/release/librdp_decoder.a
// The Windows system libraries below are required by the Rust std library that
// gets bundled into librdp_decoder.a. The list comes from:
//   cargo rustc --release --target x86_64-pc-windows-gnu -p rdp-decoder \
//     --crate-type staticlib -- --print native-static-libs
#cgo windows LDFLAGS: -lkernel32 -lntdll -luserenv -lws2_32 -ldbghelp
*/
import "C"
