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

package local

import (
	"context"
	"crypto/x509"

	"github.com/gravitational/trace"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	recordingEncryptionPrefix = "recording_encryption"
	rotatedKeyPrefix          = "recording_encryption_rotated"
)

// RecordingEncryptionService exposes backend functionality for working with the
// cluster's RecordingEncryption resource.
type RecordingEncryptionService struct {
	encryption *generic.ServiceWrapper[*recordingencryptionv1.RecordingEncryption]
	rotatedKey *generic.ServiceWrapper[*recordingencryptionv1.RotatedKey]
}

var _ services.RecordingEncryption = (*RecordingEncryptionService)(nil)

// NewRecordingEncryptionService creates a new RecordingEncryptionService.
func NewRecordingEncryptionService(b backend.Backend) (*RecordingEncryptionService, error) {
	const pageLimit = 100
	encryption, err := generic.NewServiceWrapper(generic.ServiceConfig[*recordingencryptionv1.RecordingEncryption]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindRecordingEncryption,
		BackendPrefix: backend.NewKey(recordingEncryptionPrefix),
		MarshalFunc:   services.MarshalProtoResource[*recordingencryptionv1.RecordingEncryption],
		UnmarshalFunc: services.UnmarshalProtoResource[*recordingencryptionv1.RecordingEncryption],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rotatedKey, err := generic.NewServiceWrapper(generic.ServiceConfig[*recordingencryptionv1.RotatedKey]{
		Backend:       b,
		PageLimit:     pageLimit,
		ResourceKind:  types.KindRotatedKey,
		BackendPrefix: backend.NewKey(rotatedKeyPrefix),
		MarshalFunc:   services.MarshalProtoResource[*recordingencryptionv1.RotatedKey],
		UnmarshalFunc: services.UnmarshalProtoResource[*recordingencryptionv1.RotatedKey],
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RecordingEncryptionService{
		encryption: encryption,
		rotatedKey: rotatedKey,
	}, nil
}

// CreateRecordingEncryption creates a new RecordingEncryption in the backend.
func (s *RecordingEncryptionService) CreateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error) {
	if encryption.Metadata == nil {
		encryption.Metadata = &headerv1.Metadata{}
	}
	encryption.Metadata.Name = types.MetaNameRecordingEncryption
	encryption.Kind = types.KindRecordingEncryption
	created, err := s.encryption.CreateResource(ctx, encryption)
	return created, trace.Wrap(err)
}

// UpdateRecordingEncryption replaces the RecordingEncryption resource with the given one.
func (s *RecordingEncryptionService) UpdateRecordingEncryption(ctx context.Context, encryption *recordingencryptionv1.RecordingEncryption) (*recordingencryptionv1.RecordingEncryption, error) {
	if encryption.Metadata == nil {
		encryption.Metadata = &headerv1.Metadata{}
	}
	encryption.Metadata.Name = types.MetaNameRecordingEncryption
	encryption.Kind = types.KindRecordingEncryption
	updated, err := s.encryption.ConditionalUpdateResource(ctx, encryption)
	return updated, trace.Wrap(err)
}

// DeleteRecordingEncryption removes the RecordingEncryption from the cluster.
func (s *RecordingEncryptionService) DeleteRecordingEncryption(ctx context.Context) error {
	return trace.Wrap(s.encryption.DeleteResource(ctx, types.MetaNameRecordingEncryption))
}

// GetRecordingEncryption retrieves the RecordingEncryption for the cluster.
func (s *RecordingEncryptionService) GetRecordingEncryption(ctx context.Context) (*recordingencryptionv1.RecordingEncryption, error) {
	encryption, err := s.encryption.GetResource(ctx, types.MetaNameRecordingEncryption)
	return encryption, trace.Wrap(err)
}

// CreateRotatedKey creates a new RotatedKey in the backend keyed on the fingerprint of the given public key.
func (s *RecordingEncryptionService) CreateRotatedKey(ctx context.Context, key *types.EncryptionKeyPair) (*recordingencryptionv1.RotatedKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(key.PublicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fp, err := recordingencryption.Fingerprint(parsed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := s.rotatedKey.CreateResource(ctx, &recordingencryptionv1.RotatedKey{
		Metadata: &headerv1.Metadata{
			Name: fp,
		},
		Kind: types.KindRotatedKey,
		Spec: &recordingencryptionv1.RotatedKeySpec{
			EncryptionKeyPair: key,
		},
	})
	return created, trace.Wrap(err)
}

// GetRotatedKey retrieves the RotatedKey related to the given public key fingerprint from the backend.
func (s *RecordingEncryptionService) GetRotatedKey(ctx context.Context, fingerprint string) (*recordingencryptionv1.RotatedKey, error) {
	rotatedKey, err := s.rotatedKey.GetResource(ctx, fingerprint)
	return rotatedKey, trace.Wrap(err)
}

// DeleteRotatedKey removes the RotatedKey related with the given public key fingerprint from the backend.
func (s *RecordingEncryptionService) DeleteRotatedKey(ctx context.Context, fingerprint string) error {
	return trace.Wrap(s.rotatedKey.DeleteResource(ctx, fingerprint))
}

type recordingEncryptionParser struct {
	baseParser
}

func newRecordingEncryptionParser() *recordingEncryptionParser {
	return &recordingEncryptionParser{
		baseParser: newBaseParser(backend.NewKey(recordingEncryptionPrefix)),
	}
}

func (p *recordingEncryptionParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		recordingEncryption, err := services.UnmarshalProtoResource[*recordingencryptionv1.RecordingEncryption](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}
		return types.Resource153ToLegacy(recordingEncryption), nil
	case types.OpDelete:
		return &types.ResourceHeader{
			Kind:    types.KindRecordingEncryption,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: types.MetaNameRecordingEncryption,
			},
		}, nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}

type rotatedKeyParser struct {
	baseParser
}

func newRotatedKeyParser() *rotatedKeyParser {
	return &rotatedKeyParser{
		baseParser: newBaseParser(backend.NewKey(rotatedKeyPrefix)),
	}
}

func (p *rotatedKeyParser) parse(event backend.Event) (types.Resource, error) {
	switch event.Type {
	case types.OpPut:
		rotatedKey, err := services.UnmarshalProtoResource[*recordingencryptionv1.RotatedKey](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling resource from event")
		}

		return types.Resource153ToLegacy(rotatedKey), nil
	case types.OpDelete:
		header, err := services.UnmarshalProtoResource[*headerv1.ResourceHeader](
			event.Item.Value,
			services.WithExpires(event.Item.Expires),
			services.WithRevision(event.Item.Revision),
		)
		if err != nil {
			return nil, trace.Wrap(err, "unmarshaling deleted resource header")
		}
		return types.Resource153ToLegacy(header), nil
	default:
		return nil, trace.BadParameter("event %v is not supported", event.Type)
	}
}
