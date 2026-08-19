// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

// Package user provides a thin concurrency safe wrapper around the
// `os/user` APIs for Linux when CGO is enabled.
//
// On Linux, the `os/user` package may call into C-based NSS backends via cgo.
// Some NSS implementations (or configurations) are not safe for concurrent
// use and can deadlock or crash when lookups are performed concurrently.
//
// To avoid such failures, this package serializes calls to a small set of `os/user`
// APIs on linux+cgo builds.
// Non-Linux or non-cgo builds forward calls to the stdlib without locking.
//
// References:
//
//   - Debian bug report: https://bugs.debian.org/cgi-bin/bugreport.cgi?bug=831390
//   - Teleport issue: https://github.com/gravitational/teleport/issues/69662
//
// Note that this package is placed in `session/host` specifically because the `session` tree
// is effectively a submodule within Teleport which guards against any imports from Teleport itself.
// This was done to optimize the session helper load times and avoid bleeding runtime dependencies into the
// session helper which would incease the load time and degrade latency. Since Teleport still calls
// into the session helper code in the same process for which a mutex must be shared, the user package
// is placed here despite less than ideal code organization.
package user

// TODO(okraport): revisit this module placement and investigate moving it to lib/utils/user if possible.
// TODO(okraport): revisit this module to consider replacement with getent process shell out to avoid the mutex.
