/**
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package recordingdetailsv1

import (
	"context"

	"github.com/gravitational/trace"

	recordingdetailsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingdetails/v1"
)

// NewService creates a new OSS version of the SummarizerService. It
// returns a licensing error from every RPC.
func NewService() *UnimplementedService {
	return &UnimplementedService{}
}

// UnimplementedService is an OSS version of the UnimplementedService. It
// returns a licensing error from every RPC.
type UnimplementedService struct {
	recordingdetailsv1pb.UnimplementedRecordingDetailsServiceServer
}

// GetThumbnail is supposed to get a session recording's thumbnail, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) GetThumbnail(
	ctx context.Context, req *recordingdetailsv1pb.GetThumbnailRequest,
) (*recordingdetailsv1pb.GetThumbnailResponse, error) {
	return nil, requireEnterprise()
}

// GetDetails is supposed to get session recording details, but
// returns an error indicating that this feature is only available in the
// enterprise version of Teleport.
func (s *UnimplementedService) GetDetails(
	ctx context.Context, req *recordingdetailsv1pb.GetDetailsRequest,
) (*recordingdetailsv1pb.GetDetailsResponse, error) {
	return nil, requireEnterprise()
}

func requireEnterprise() error {
	return trace.AccessDenied(
		"session details and thumbnails are only available with an enterprise license that supports Teleport Identity Security")
}
