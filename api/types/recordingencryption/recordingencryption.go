// Copyright 2026 Gravitational, Inc.
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

package recordingencryption

import (
	"google.golang.org/protobuf/proto"

	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
)

// CopyEncryptionWithoutSecrets returns a copy of a recording encryption without private keys.
func CopyEncryptionWithoutSecrets(r *recordingencryptionv1.RecordingEncryption) *recordingencryptionv1.RecordingEncryption {
	r1 := proto.CloneOf(r)
	spec := r1.GetSpec()
	if spec == nil {
		return r1
	}
	for _, outerKeyPair := range spec.GetActiveKeyPairs() {
		keyPair := outerKeyPair.GetKeyPair()
		if keyPair != nil {
			keyPair.PrivateKey = nil
		}
	}
	return r1
}

// CopyRotatedKey returns a copy of a rotated key without private keys.
func CopyRotatedKeyWithoutSecrets(r *recordingencryptionv1.RotatedKey) *recordingencryptionv1.RotatedKey {
	r1 := proto.CloneOf(r)
	spec := r1.GetSpec()
	if spec == nil {
		return r1
	}
	if keypair := spec.GetEncryptionKeyPair(); keypair != nil {
		keypair.PrivateKey = nil
	}
	return r1
}
