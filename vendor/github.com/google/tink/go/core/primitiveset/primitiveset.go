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

// Package primitiveset provides a container for a set of cryptographic
// primitives.
//
// It provides also additional properties for the primitives it holds. In
// particular, one of the primitives in the set can be distinguished as "the
// primary" one.
package primitiveset

import (
	"fmt"

	"github.com/google/tink/go/core/cryptofmt"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

// Entry represents a single entry in the keyset. In addition to the actual
// primitive, it holds the identifier and status of the primitive.
type Entry struct {
	KeyID      uint32
	Primitive  interface{}
	Prefix     string
	PrefixType tinkpb.OutputPrefixType
	Status     tinkpb.KeyStatusType
}

func newEntry(keyID uint32, p interface{}, prefix string, prefixType tinkpb.OutputPrefixType, status tinkpb.KeyStatusType) *Entry {
	return &Entry{
		KeyID:      keyID,
		Primitive:  p,
		Prefix:     prefix,
		Status:     status,
		PrefixType: prefixType,
	}
}

// PrimitiveSet is used for supporting key rotation: primitives in a set
// correspond to keys in a keyset. Users will usually work with primitive
// instances, which essentially wrap primitive sets. For example an instance of
// an AEAD-primitive for a given keyset holds a set of AEAD-primitives
// corresponding to the keys in the keyset, and uses the set members to do the
// actual crypto operations: to encrypt data the primary AEAD-primitive from
// the set is used, and upon decryption the ciphertext's prefix determines the
// id of the primitive from the set.
//
// PrimitiveSet is a public to allow its use in implementations of custom
// primitives.
type PrimitiveSet struct {
	// Primary entry.
	Primary *Entry

	// The primitives are stored in a map of (ciphertext prefix, list of
	// primitives sharing the prefix). This allows quickly retrieving the
	// primitives sharing some particular prefix.
	Entries map[string][]*Entry
}

// New returns an empty instance of PrimitiveSet.
func New() *PrimitiveSet {
	return &PrimitiveSet{
		Primary: nil,
		Entries: make(map[string][]*Entry),
	}
}

// RawEntries returns all primitives in the set that have RAW prefix.
func (ps *PrimitiveSet) RawEntries() ([]*Entry, error) {
	return ps.EntriesForPrefix(cryptofmt.RawPrefix)
}

// EntriesForPrefix returns all primitives in the set that have the given prefix.
func (ps *PrimitiveSet) EntriesForPrefix(prefix string) ([]*Entry, error) {
	result, found := ps.Entries[prefix]
	if !found {
		return []*Entry{}, nil
	}
	return result, nil
}

// Add creates a new entry in the primitive set and returns the added entry.
func (ps *PrimitiveSet) Add(p interface{}, key *tinkpb.Keyset_Key) (*Entry, error) {
	if key == nil || p == nil {
		return nil, fmt.Errorf("primitive_set: key and primitive must not be nil")
	}
	if key.Status != tinkpb.KeyStatusType_ENABLED {
		return nil, fmt.Errorf("The key must be ENABLED")
	}
	prefix, err := cryptofmt.OutputPrefix(key)
	if err != nil {
		return nil, fmt.Errorf("primitive_set: %s", err)
	}
	e := newEntry(key.KeyId, p, prefix, key.OutputPrefixType, key.Status)
	ps.Entries[prefix] = append(ps.Entries[prefix], e)
	return e, nil
}
