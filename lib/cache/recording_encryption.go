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

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recencpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/services"
)

type recordingEncryptionIndex string

const recordingEncryptionNameIndex recordingEncryptionIndex = "name"

func newRecordingEncryptionCollection(upstream services.RecordingEncryption, w types.WatchKind) (*collection[*recencpb.RecordingEncryption, recordingEncryptionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter RecordingEncryption")
	}

	return &collection[*recencpb.RecordingEncryption, recordingEncryptionIndex]{
		store: newStore(utils.CloneProtoMsg[*recencpb.RecordingEncryption], map[recordingEncryptionIndex]func(*recencpb.RecordingEncryption) string{
			recordingEncryptionNameIndex: func(r *recencpb.RecordingEncryption) string {
				return r.GetMetadata().GetName()
			},
		}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*recencpb.RecordingEncryption, error) {
			recordingEncryption, err := upstream.GetRecordingEncryption(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			return []*recencpb.RecordingEncryption{recordingEncryption}, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *recencpb.RecordingEncryption {
			return &recencpb.RecordingEncryption{
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

// GetRecordingEncryption returns the cached RecordingEncryption for the cluster
func (c *Cache) GetRecordingEncryption(ctx context.Context) (*recencpb.RecordingEncryption, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetRecordingEncryption")
	defer span.End()

	getter := genericGetter[*recencpb.RecordingEncryption, recordingEncryptionIndex]{
		cache:      c,
		collection: c.collections.recordingEncryption,
		index:      recordingEncryptionNameIndex,
		upstreamGet: func(ctx context.Context, ident string) (*recencpb.RecordingEncryption, error) {
			return c.Config.RecordingEncryption.GetRecordingEncryption(ctx)
		},
	}

	out, err := getter.get(ctx, types.MetaNameRecordingEncryption)
	return out, trace.Wrap(err)
}
