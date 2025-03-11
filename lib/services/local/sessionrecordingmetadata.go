/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package local

import (
	"context"
	sessionrecordingmetatadav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionrecordingmetatada/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
	"slices"
)

// SessionRecordingMetadataService manages session recording metadata resources in the backend.
type SessionRecordingMetadataService struct {
	service *generic.ServiceWrapper[*sessionrecordingmetatadav1.SessionRecordingMetadata]
}

func (s SessionRecordingMetadataService) CreateSessionRecordingMetadata(ctx context.Context, metadata *sessionrecordingmetatadav1.SessionRecordingMetadata) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	return s.service.CreateResource(ctx, metadata)
}

func (s SessionRecordingMetadataService) UpdateSessionRecordingMetadata(ctx context.Context, metadata *sessionrecordingmetatadav1.SessionRecordingMetadata) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	return s.service.ConditionalUpdateResource(ctx, metadata)
}

func (s SessionRecordingMetadataService) GetSessionRecordingMetadata(ctx context.Context, sessionID string) (*sessionrecordingmetatadav1.SessionRecordingMetadata, error) {
	return s.service.GetResource(ctx, sessionID)
}

func (s SessionRecordingMetadataService) DeleteSessionRecordingMetadata(ctx context.Context, sessionID string) error {
	return s.service.DeleteResource(ctx, sessionID)
}

func (s SessionRecordingMetadataService) ListSessionRecordingMetadata(ctx context.Context, pageSize int, nextToken string, sessionIDs []string, withSummary bool) ([]*sessionrecordingmetatadav1.SessionRecordingMetadata, string, error) {
	return s.service.ListResourcesWithFilter(ctx, pageSize, nextToken, func(metadata *sessionrecordingmetatadav1.SessionRecordingMetadata) bool {
		return (len(sessionIDs) == 0 || slices.Contains(sessionIDs, metadata.Metadata.Name)) && (!withSummary || len(metadata.Spec.GetSummary()) > 0)
	})
}

// NewSessionRecordingMetadataService creates a new WindowsDesktopsService.
func NewSessionRecordingMetadataService(b backend.Backend) (*SessionRecordingMetadataService, error) {
	service, err := generic.NewServiceWrapper(generic.ServiceWrapperConfig[*sessionrecordingmetatadav1.SessionRecordingMetadata]{
		Backend:       b,
		ResourceKind:  types.KindSessionRecordingMetadata,
		PageLimit:     defaults.MaxIterationLimit,
		BackendPrefix: backend.NewKey(sessionRecordingMetadataPrefix),
		MarshalFunc:   services.MarshalSessionRecordingMetadata,
		UnmarshalFunc: services.UnmarshalSessionRecordingMetadata,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SessionRecordingMetadataService{
		service: service,
	}, nil
}

const (
	sessionRecordingMetadataPrefix = "sessionRecordingMetadata"
)
