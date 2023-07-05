//go:build protogen && !protogen

// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package teleport

// This file is needed to keep a dependency on the go protogen tools we `go run`
// during our code generation - especially protoc-gen-go-grpc, as that's a
// separate go module than google.golang.org/grpc (on which we depend
// "naturally")

import (
	_ "github.com/bufbuild/connect-go/cmd/protoc-gen-connect-go"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
