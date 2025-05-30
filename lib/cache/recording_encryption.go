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

//nolint:unused // Because the executors generate a large amount of false positives.
package cache

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type recordingEncryptionIndex string

const recordingEncryptionNameIndex recordingEncryptionIndex = "name"

func newRecordingEncryptionCollection(upstream services.RecordingEncryption, w types.WatchKind) (*collection[*recordingencryptionv1.RecordingEncryption, recordingEncryptionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter RecordingEncryption")
	}

	return &collection[*recordingencryptionv1.RecordingEncryption, recordingEncryptionIndex]{
		store: newStore(proto.CloneOf[*recordingencryptionv1.RecordingEncryption], map[recordingEncryptionIndex]func(*recordingencryptionv1.RecordingEncryption) string{
			recordingEncryptionNameIndex: func(r *recordingencryptionv1.RecordingEncryption) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*recordingencryptionv1.RecordingEncryption, error) {
			recordingEncryption, err := upstream.GetRecordingEncryption(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []*recordingencryptionv1.RecordingEncryption{recordingEncryption}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *recordingencryptionv1.RecordingEncryption {
			return &recordingencryptionv1.RecordingEncryption{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

type recordingEncryptionCacheKey struct {
	kind string
}

// GetRecordingEncryption returns the cached RecordingEncryption for the cluster
func (c *Cache) GetRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRecordingEncryption")
	defer span.End()

	getter := genericGetter[*recordingencryptionv1.RecordingEncryption, recordingEncryptionIndex]{
		cache:      c,
		collection: c.collections.recordingEncryption,
		index:      recordingEncryptionNameIndex,
		upstreamGet: func(ctx context.Context, ident string) (*recordingencryptionv1.RecordingEncryption, error) {
			encryption, err := c.Config.RecordingEncryption.GetRecordingEncryption(ctx)
			return encryption, trace.Wrap(err)
		},
	}

	out, err := getter.get(ctx, types.MetaNameRecordingEncryption)
	return out, trace.Wrap(err)
}
