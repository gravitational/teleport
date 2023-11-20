//go:build !cgo

// Copyright 2023 Gravitational, Inc
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

package webauthncli

import (
	"context"
	"github.com/gravitational/trace"
)

// RunOnU2FDevices stubs out RunOnU2FDevices when performing builds without
// CGO. This disables support for U2F devices.
func RunOnU2FDevices(ctx context.Context, runCredentials ...func(Token) error) error {
	return trace.NotImplemented("u2f support requires cgo")
}
