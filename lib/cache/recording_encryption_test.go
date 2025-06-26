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
	"context"
	"testing"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
)

func newRecordingEncryption() *recordingencryptionv1.RecordingEncryption {
	return &recordingencryptionv1.RecordingEncryption{
		Kind:    types.KindRecordingEncryption,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: types.MetaNameRecordingEncryption,
		},
		Spec: &recordingencryptionv1.RecordingEncryptionSpec{
			ActiveKeys: nil,
		},
	}
}

// TestRecordingEncryption tests that CRUD operations on the RecordingEncryption resource are
// replicated from the backend to the cache.
func TestRecordingEncryption(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources153(t, p, testFuncs153[*recordingencryptionv1.RecordingEncryption]{
		newResource: func(name string) (*recordingencryptionv1.RecordingEncryption, error) {
			return newRecordingEncryption(), nil
		},
		create: func(ctx context.Context, item *recordingencryptionv1.RecordingEncryption) error {
			_, err := p.recordingEncryption.CreateRecordingEncryption(ctx, item)
			return trace.Wrap(err)
		},
		update: func(ctx context.Context, item *recordingencryptionv1.RecordingEncryption) error {
			_, err := p.recordingEncryption.UpdateRecordingEncryption(ctx, item)
			return trace.Wrap(err)
		},
		list: func(ctx context.Context) ([]*recordingencryptionv1.RecordingEncryption, error) {
			item, err := p.recordingEncryption.GetRecordingEncryption(ctx)
			if trace.IsNotFound(err) {
				return []*recordingencryptionv1.RecordingEncryption{}, nil
			}
			return []*recordingencryptionv1.RecordingEncryption{item}, trace.Wrap(err)
		},
		cacheList: func(ctx context.Context) ([]*recordingencryptionv1.RecordingEncryption, error) {
			item, err := p.cache.GetRecordingEncryption(ctx)
			if trace.IsNotFound(err) {
				return []*recordingencryptionv1.RecordingEncryption{}, nil
			}
			return []*recordingencryptionv1.RecordingEncryption{item}, trace.Wrap(err)
		},
		deleteAll: func(ctx context.Context) error {
			return trace.Wrap(p.recordingEncryption.DeleteRecordingEncryption(ctx))
		},
	})
}
