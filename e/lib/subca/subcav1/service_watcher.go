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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/subca"
)

func (s *Service) runCAOverrideWatcher(ctx context.Context) {
	w, err := s.newCAOverrideWatcher(
		ctx,
		"subca-watcher",
		func(e *types.Event) {
			s.updateOverridesStatus(ctx)
		},
		func(op types.OpType, caOverride *subcav1.CertAuthorityOverride) {
			if op == types.OpPut {
				s.updateOverrideStatus(ctx, caOverride)
			}
		},
	)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to create CA override watcher", "error", err)
		return
	}
	w.run()
}

func (s *Service) updateOverridesStatus(ctx context.Context) {
	caOverrides, err := stream.Collect(
		clientutils.Resources(ctx, s.cachedSubCA.ListCertAuthorityOverrides))
	if err != nil {
		s.logger.WarnContext(ctx,
			"Failed to list CA overrides for watcher-based updates. Async status updates may have a gap.",
			"error", err,
		)
		return
	}
	for _, caOverride := range caOverrides {
		s.logger.DebugContext(ctx, "Trigger CA override status update on watcher init",
			"ca_type", caOverride.GetSubKind(),
			"cluster_name", caOverride.GetMetadata().GetName(),
		)
		s.updateOverrideStatus(ctx, caOverride)
	}
}

func (s *Service) updateOverrideStatus(ctx context.Context, caOverride *subcav1.CertAuthorityOverride) {
	// Take a defensive copy before triggering potential modifications.
	caOverride = proto.CloneOf(caOverride)

	if err := s.conditionalUpdateOverrideStatus(ctx, caOverride); err != nil {
		s.logger.WarnContext(ctx, "Async CA override status update failed",
			"error", err,
			"ca_type", caOverride.GetSubKind(),
			"cluster_name", caOverride.GetMetadata().GetName(),
		)
	}
}

func (s *Service) conditionalUpdateOverrideStatus(ctx context.Context, initial *subcav1.CertAuthorityOverride) error {
	id := local.CertAuthorityOverrideIDFromResource(initial)
	generatedCRLCache := make(map[string]*subcav1.CertificateRevocationList)
	now := s.clock.Now()

	const loadKeys = true
	getParsedCA := s.getParsedCAOnce(ctx, types.CertAuthID{
		Type:       types.CertAuthType(id.CAType),
		DomainName: id.ClusterName,
	}, loadKeys)

	logger := s.logger.With(
		"ca_type", initial.GetSubKind(),
		"cluster_name", initial.GetMetadata().GetName(),
	)

	_, err := s.conditionalUpdateWithRetry(
		ctx,
		id,
		initial,
		func(caOverride *subcav1.CertAuthorityOverride) (done bool, _ error) {
			parsed, err := subca.ParseCAOverride(caOverride)
			if err != nil {
				return false, trace.Wrap(err, "parse CA override")
			}
			done = true // Assume no updates needed.

			// CRLs.
			changed, err := s.updateOverrideStatusCRLs(ctx, getParsedCA, parsed, generatedCRLCache, now)
			if err != nil {
				logger.WarnContext(ctx, "Failed to update CA override CRLs", "error", err)
			} else {
				done = done && !changed
			}

			return done, nil
		})
	return trace.Wrap(err)
}
