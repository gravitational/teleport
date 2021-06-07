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

package registry

import (
	"github.com/golang/protobuf/proto"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

// KeyManager "understands" keys of a specific key types: it can generate keys of a supported type
// and create primitives for supported keys.  A key type is identified by the global name of the
// protocol buffer that holds the corresponding key material, and is given by type_url-field of
// KeyData-protocol buffer.
type KeyManager interface {
	// Primitive constructs a primitive instance for the key given in serializedKey, which must be a
	// serialized key protocol buffer handled by this manager.
	Primitive(serializedKey []byte) (interface{}, error)

	// NewKey generates a new key according to specification in serializedKeyFormat, which must be
	// supported by this manager.
	NewKey(serializedKeyFormat []byte) (proto.Message, error)

	// DoesSupport returns true iff this KeyManager supports key type identified by typeURL.
	DoesSupport(typeURL string) bool

	// TypeURL returns the type URL that identifes the key type of keys managed by this key manager.
	TypeURL() string

	// APIs for Key Management

	// NewKeyData generates a new KeyData according to specification in serializedkeyFormat.
	// This should be used solely by the key management API.
	NewKeyData(serializedKeyFormat []byte) (*tinkpb.KeyData, error)
}
