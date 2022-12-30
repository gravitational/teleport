// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package webauthncli provides the client-side implementation for WebAuthn.
//
// There are two separate implementations contained within the package:
//
// * A U2F (aka CTAP1), pure Go implementation, backed by flynn/u2f
// * A FIDO2 (aka CTAP1/CTAP2) implementation, backed by Yubico's libfido2
//
// High-level API methods prefer the FIDO2 implementation, falling back to U2F
// if the binary isn't compiled with libfido2 support. Note that passwordless
// requires CTAP2.
//
// The FIDO2 implementation is protected by the `libfido2` build tag. In order
// to build FIDO2-enabled code, in addition to setting the build tag (eg, `go
// build -tags=libfido2 ./tool/tsh` or `go test -tags=libfido2
// ./lib/auth/webauthncli`), you must first have libfido2 installed in your
// system.
//
// To install libfido2 [follow its installation instructions](
// https://github.com/Yubico/libfido2#installation). See
// [gravitational/go-libfido2](https://github.com/gravitational/go-libfido2#why-fork)
// for additional build options.
//
// Refer to
// [buildbox](https://github.com/gravitational/teleport/blob/master/build.assets/Dockerfile#L10)
// for the library versions used by release binaries.
package webauthncli
