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

package keyset

import (
	"fmt"

	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

// ValidateKeyVersion checks whether the given version is valid. The version is valid
// only if it is the range [0..maxExpected]
func ValidateKeyVersion(version, maxExpected uint32) error {
	if version > maxExpected {
		return fmt.Errorf("key has version %v; only keys with version in range [0..%v] are supported",
			version, maxExpected)
	}
	return nil
}

// Validate validates the given key set.
// Returns nil if it is valid; an error otherwise.
func Validate(keyset *tinkpb.Keyset) error {
	if keyset == nil {
		return fmt.Errorf("Validate() called with nil")
	}
	if len(keyset.Key) == 0 {
		return fmt.Errorf("empty keyset")
	}
	primaryKeyID := keyset.PrimaryKeyId
	hasPrimaryKey := false
	containsOnlyPub := true
	numEnabledKeys := 0
	for _, key := range keyset.Key {
		if err := validateKey(key); err != nil {
			return err
		}
		if key.Status != tinkpb.KeyStatusType_ENABLED {
			continue
		}
		if key.KeyId == primaryKeyID {
			if hasPrimaryKey {
				return fmt.Errorf("keyset contains multiple primary keys")
			}
			hasPrimaryKey = true
		}
		if key.KeyData.KeyMaterialType != tinkpb.KeyData_ASYMMETRIC_PUBLIC {
			containsOnlyPub = false
		}
		numEnabledKeys++
	}
	if numEnabledKeys == 0 {
		return fmt.Errorf("keyset must contain at least one ENABLED key")
	}

	if !hasPrimaryKey && !containsOnlyPub {
		return fmt.Errorf("keyset does not contain a valid primary key")
	}
	return nil
}

/*
validateKey validates the given key.
Returns nil if it is valid; an error otherwise.
*/
func validateKey(key *tinkpb.Keyset_Key) error {
	if key == nil {
		return fmt.Errorf("ValidateKey() called with nil")
	}
	if key.KeyId == 0 {
		return fmt.Errorf("key has zero key id: %d", key.KeyId)
	}
	if key.KeyData == nil {
		return fmt.Errorf("key %d has no key data", key.KeyId)
	}
	if key.OutputPrefixType != tinkpb.OutputPrefixType_TINK &&
		key.OutputPrefixType != tinkpb.OutputPrefixType_LEGACY &&
		key.OutputPrefixType != tinkpb.OutputPrefixType_RAW &&
		key.OutputPrefixType != tinkpb.OutputPrefixType_CRUNCHY {
		return fmt.Errorf("key %d has unknown prefix", key.KeyId)
	}
	if key.Status != tinkpb.KeyStatusType_ENABLED &&
		key.Status != tinkpb.KeyStatusType_DISABLED &&
		key.Status != tinkpb.KeyStatusType_DESTROYED {
		return fmt.Errorf("key %d has unknown status", key.KeyId)
	}
	return nil
}
