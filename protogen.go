//go:build protogen && !protogen

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package teleport

// This file is needed to keep a dependency on the go protogen tools we `go run`
// during our code generation - especially protoc-gen-go-grpc, as that's a
// separate go module than google.golang.org/grpc (on which we depend
// "naturally")

import (
	_ "connectrpc.com/connect/cmd/protoc-gen-connect-go"
	_ "github.com/gogo/protobuf/protoc-gen-gogofast"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
