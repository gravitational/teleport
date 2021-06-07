// Copyright 2018 Google LLC
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
//
////////////////////////////////////////////////////////////////////////////////

// Package keyset provides methods to generate, read, write or validate
// keysets.
package keyset

import (
	"github.com/google/tink/go/internal"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

// keysetHandle is used by package insecurecleartextkeyset and package
// testkeyset (via package internal) to create a keyset.Handle from cleartext
// key material.
func keysetHandle(ks *tinkpb.Keyset) *Handle {
	return &Handle{ks}
}

// keysetMaterial is used by package insecurecleartextkeyset and package
// testkeyset (via package internal) to read the key material in a
// keyset.Handle.
func keysetMaterial(h *Handle) *tinkpb.Keyset {
	return h.ks
}

func init() {
	internal.KeysetHandle = keysetHandle
	internal.KeysetMaterial = keysetMaterial
}
