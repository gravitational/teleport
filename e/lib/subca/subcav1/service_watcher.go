// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package subcav1

import (
	"context"

	"google.golang.org/protobuf/proto"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

func (s *Service) runCAOverrideWatcher(ctx context.Context) {
	w, err := s.newCAOverrideWatcher(
		ctx,
		"subca-watcher",
		func(e *types.Event) {
			s.triggerUpdateAllCRLs(ctx)
		},
		func(op types.OpType, caOverride *subcav1.CertAuthorityOverride) {
			if op == types.OpPut {
				s.triggerUpdateOverrideCRLs(ctx, caOverride)
			}
		},
	)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to create CA override watcher", "error", err)
		return
	}
	w.run()
}

func (s *Service) triggerUpdateAllCRLs(ctx context.Context) {
	caOverrides, err := stream.Collect(
		clientutils.Resources(ctx, s.cachedSubCA.ListCertAuthorityOverrides))
	if err != nil {
		s.logger.WarnContext(ctx,
			"Failed to list CA overrides for watcher-based updates. Async CRL generation may have a gap.",
			"error", err,
		)
		return
	}
	for _, caOverride := range caOverrides {
		s.triggerUpdateOverrideCRLs(ctx, caOverride)
	}
}

func (s *Service) triggerUpdateOverrideCRLs(ctx context.Context, caOverride *subcav1.CertAuthorityOverride) {
	// Take a defensive copy before triggering potential modifications.
	caOverride = proto.CloneOf(caOverride)

	s.logger.DebugContext(ctx, "Async CA override CRL update triggered",
		"ca_override", caOverride,
	)

	if err := s.updateOverrideCRLs(ctx, caOverride); err != nil {
		s.logger.WarnContext(ctx, "Async CA override CRL update failed",
			"error", err,
			"ca_type", caOverride.GetSubKind(),
			"cluster_name", caOverride.GetMetadata().GetName(),
		)
	}
}
