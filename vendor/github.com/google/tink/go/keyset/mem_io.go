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

// MemReaderWriter implements keyset.Reader and keyset.Writer for *tinkpb.Keyset and *tinkpb.EncryptedKeyset.
type MemReaderWriter struct {
	Keyset          *tinkpb.Keyset
	EncryptedKeyset *tinkpb.EncryptedKeyset
}

// MemReaderWriter implements Reader and Writer.
var _ Reader = &MemReaderWriter{}
var _ Writer = &MemReaderWriter{}

// Read returns *tinkpb.Keyset from memory.
func (m *MemReaderWriter) Read() (*tinkpb.Keyset, error) {
	return m.Keyset, nil
}

// ReadEncrypted returns *tinkpb.EncryptedKeyset from memory.
func (m *MemReaderWriter) ReadEncrypted() (*tinkpb.EncryptedKeyset, error) {
	return m.EncryptedKeyset, nil
}

// Write keyset to memory.
func (m *MemReaderWriter) Write(keyset *tinkpb.Keyset) error {
	m.Keyset = keyset
	return nil
}

// WriteEncrypted keyset to memory.
func (m *MemReaderWriter) WriteEncrypted(keyset *tinkpb.EncryptedKeyset) error {
	m.EncryptedKeyset = keyset
	return nil
}
