// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"testing"
	"testing/synctest"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
)

func newRecordingEncryption() *recordingencryptionv1.RecordingEncryption {
	return recordingencryptionv1.RecordingEncryption_builder{
		Kind:    types.KindRecordingEncryption,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: types.MetaNameRecordingEncryption,
		}.Build(),
		Spec: recordingencryptionv1.RecordingEncryptionSpec_builder{
			ActiveKeyPairs: nil,
		}.Build(),
	}.Build()
}

// TestRecordingEncryption tests that CRUD operations on the RecordingEncryption resource are
// replicated from the backend to the cache.
func TestRecordingEncryption(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newTestPack(t, ForAuth)
		t.Cleanup(p.Close)
		testSingleton153(t, p, testSingletonFuncs153[*recordingencryptionv1.RecordingEncryption]{
			newResource: newRecordingEncryption,
			create:      p.recordingEncryption.CreateRecordingEncryption,
			update:      p.recordingEncryption.UpdateRecordingEncryption,
			get:         p.recordingEncryption.GetRecordingEncryption,
			cacheGet:    p.cache.GetRecordingEncryption,
			delete:      p.recordingEncryption.DeleteRecordingEncryption,
		})
	})

}
