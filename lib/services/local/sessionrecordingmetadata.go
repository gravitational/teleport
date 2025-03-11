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
	sessionrecordingmetatadav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionrecordingmetatada/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
	"github.com/gravitational/trace"
)

// SessionRecordingMetadataService manages session recording metadata resources in the backend.
type SessionRecordingMetadataService struct {
	service *generic.ServiceWrapper[*sessionrecordingmetatadav1.SessionRecordingMetadata]
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
