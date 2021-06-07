// Copyright 2019 Google LLC
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

package keyset

import tinkpb "github.com/google/tink/go/proto/tink_go_proto"

// Reader knows how to read a Keyset or an EncryptedKeyset from some source.
// In order to turn a Reader into a KeysetHandle for use, callers must use
// insecure.KeysetHandle or by keyset.Read (with encryption).
type Reader interface {
	// Read returns a (cleartext) Keyset object from the underlying source.
	Read() (*tinkpb.Keyset, error)

	// ReadEncrypted returns an EncryptedKeyset object from the underlying source.
	ReadEncrypted() (*tinkpb.EncryptedKeyset, error)
}
