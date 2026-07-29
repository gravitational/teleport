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
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/subca"
	"github.com/gravitational/teleport/lib/utils/log"
)

func (s *Service) runSubCAWatcher(ctx context.Context) {
	w, err := s.newResourceWatcher(
		ctx,
		types.Watch{
			Name: "subca-watcher",
			Kinds: []types.WatchKind{
				{Kind: types.KindCertAuthorityOverride},
				{Kind: types.KindPendingCSRRequest},
			},
		},
		func(*types.Event) {
			// Have the entire batch run against a consistent timestamp and CA view.
			now := s.clock.Now()
			caGetter := s.newMemoizedCAGetter()

			s.updateOverridesStatus(ctx, caGetter, now)
			s.updatePendingCSRs(ctx, caGetter)
		},
		func(op types.OpType, e *types.Event) {
			if op != types.OpPut {
				return
			}
			now := s.clock.Now()
			switch e.Resource.GetKind() {
			case types.KindCertAuthorityOverride:
				s.handleCAOverrideEvent(ctx, e, now)
			case types.KindPendingCSRRequest:
				s.handlePendingCSRRequestEvent(ctx, e, now)
			}
		},
	)
	if err != nil {
		s.logger.WarnContext(ctx, "Failed to create SubCA watcher", "error", err)
		return
	}
	w.run()
}

func (s *Service) runPendingCSRWatcher(
	ctx context.Context,
	requestID string,
	onInit func(e *types.Event),
	onEvent func(op types.OpType, pendingReq *subcav1.PendingCSRRequest),
) error {
	w, err := s.newResourceWatcher(
		ctx,
		types.Watch{
			Name: "subca-csr-" + requestID,
			Kinds: []types.WatchKind{
				{Kind: types.KindPendingCSRRequest},
			},
		},
		onInit,
		func(op types.OpType, e *types.Event) {
			if e.Resource == nil || e.Resource.GetName() != requestID {
				return
			}
			pendingReq, ok := s.pendingCSRRequestFromEvent(ctx, e)
			if !ok {
				return
			}
			onEvent(op, pendingReq)
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	w.run()
	return nil
}

func (s *Service) handleCAOverrideEvent(ctx context.Context, e *types.Event, now time.Time) {
	caOverride, ok := s.caOverrideFromEvent(ctx, e)
	if !ok {
		return
	}

	const loadKeys = true
	getParsedCA := s.getParsedCAOnce(ctx, types.CertAuthID{
		Type:       types.CertAuthType(caOverride.GetSubKind()),
		DomainName: caOverride.GetMetadata().GetName(),
	}, loadKeys)

	s.updateOverrideStatus(ctx, getParsedCA, caOverride, now)
}

func (s *Service) handlePendingCSRRequestEvent(ctx context.Context, e *types.Event, now time.Time) {
	pendingReq, ok := s.pendingCSRRequestFromEvent(ctx, e)
	if !ok {
		return
	}

	const loadKeys = true
	getParsedCA := s.getParsedCAOnce(ctx, types.CertAuthID{
		Type:       types.CertAuthType(pendingReq.GetSpec().GetCaType()),
		DomainName: pendingReq.GetSpec().GetClusterName(),
	}, loadKeys)

	s.updatePendingCSR(ctx, getParsedCA, pendingReq)
}

func (s *Service) updateOverridesStatus(
	ctx context.Context,
	caGetter *memoizedCAGetter,
	now time.Time,
) {
	for caOverride, err := range clientutils.Resources(ctx, s.cachedSubCA.ListCertAuthorityOverrides) {
		if err != nil {
			s.logger.WarnContext(ctx,
				"Failed to list CA overrides for watcher-based updates. Async status updates may have a gap.",
				"error", err,
			)
			return
		}

		id := types.CertAuthID{
			Type:       types.CertAuthType(caOverride.GetSubKind()),
			DomainName: caOverride.GetMetadata().GetName(),
		}
		getParsedCA := caGetter.getParsedCAOnce(ctx, id)

		s.logger.DebugContext(ctx, "Trigger CA override status update on watcher init",
			"ca_type", id.Type,
			"cluster_name", id.DomainName,
		)
		s.updateOverrideStatus(ctx, getParsedCA, caOverride, now)
	}
}

func (s *Service) updateOverrideStatus(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	initial *subcav1.CertAuthorityOverride,
	now time.Time,
) {
	// Take a defensive copy before triggering potential modifications.
	initial = proto.CloneOf(initial)

	id := local.CertAuthorityOverrideIDFromResource(initial)
	generatedCRLCache := make(map[string]*subcav1.CertificateRevocationList)

	logger := s.logger.With(
		"ca_type", id.CAType,
		"cluster_name", id.ClusterName,
	)

	if _, err := s.condUpdateCAOverride(
		ctx,
		id,
		initial,
		func(caOverride *subcav1.CertAuthorityOverride) (done bool, _ error) {
			parsed, err := subca.ParseCAOverride(caOverride)
			if err != nil {
				return false, trace.Wrap(err, "parse CA override")
			}

			// CRLs.
			changed, err := s.updateOverrideStatusCRLs(ctx, getParsedCA, parsed, generatedCRLCache, now)
			if err != nil {
				return false, trace.Wrap(err, "update status CRLs")
			}
			return !changed, nil
		},
	); err != nil {
		logger.WarnContext(ctx, "Async CA override status update failed",
			"error", err,
		)
	}
}

func (s *Service) updatePendingCSRs(
	ctx context.Context,
	caGetter *memoizedCAGetter,
) {
	for pendingReq, err := range clientutils.Resources(ctx, s.pendingCSR.ListPendingCSRRequests) {
		if err != nil {
			s.logger.WarnContext(ctx,
				"Failed to list PendingCSRRequests for watcher-based updates. Async CSR requests may have a gap.",
				"error", err,
			)
			return
		}

		id := types.CertAuthID{
			Type:       types.CertAuthType(pendingReq.GetSpec().GetCaType()),
			DomainName: pendingReq.GetSpec().GetClusterName(),
		}
		getParsedCA := caGetter.getParsedCAOnce(ctx, id)

		s.logger.DebugContext(ctx, "Trigger PendingCSRRequest update on watcher init",
			"ca_type", id.Type,
			"cluster_name", id.DomainName,
		)
		s.updatePendingCSR(ctx, getParsedCA, pendingReq)
	}
}

func (s *Service) updatePendingCSR(
	ctx context.Context,
	getParsedCA getParsedCAFunc,
	initial *subcav1.PendingCSRRequest,
) {
	// Take a defensive copy before triggering potential modifications.
	initial = proto.CloneOf(initial)

	logger := s.logger.With(
		"name", initial.GetMetadata().GetName(),
		"ca_type", initial.GetSpec().GetCaType(),
		"cluster_name", initial.GetSpec().GetClusterName(),
	)

	generatedCSRCache := make(map[string]*subcav1.PendingCSR)

	switch _, err := s.condUpdatePendingCSRRequest(
		ctx,
		initial.GetMetadata().GetName(),
		initial,
		func(pendingReq *subcav1.PendingCSRRequest) (done bool, _ error) {
			changed, err := s.fulfillPendingCSRRequest(ctx, getParsedCA, pendingReq, generatedCSRCache)
			if err != nil {
				return false, trace.Wrap(err)
			}
			return !changed, nil
		},
	); {
	case trace.IsNotFound(err):
		// OK, pending CSRs are deleted quickly after all requests are fulfilled,
		// which can cause watchers to see a NotFound error. No need to log it.
	case err != nil:
		logger.WarnContext(ctx, "Async PendingCSRRequest update failed",
			"error", err,
		)
	}
}

// memoizedCAGetter caches getParsedCAFunc instances per CA ID.
// Used to present a consistent CA view between multiple getParsedCAOnce calls.
type memoizedCAGetter struct {
	getParsedCAOnceFn func(_ context.Context, _ types.CertAuthID, loadKeys bool) getParsedCAFunc
	cachedGetters     map[types.CertAuthID]getParsedCAFunc
}

func (s *Service) newMemoizedCAGetter() *memoizedCAGetter {
	return &memoizedCAGetter{
		getParsedCAOnceFn: s.getParsedCAOnce,
		cachedGetters:     make(map[types.CertAuthID]getParsedCAFunc),
	}
}

func (m *memoizedCAGetter) getParsedCAOnce(ctx context.Context, id types.CertAuthID) getParsedCAFunc {
	fn, ok := m.cachedGetters[id]
	if !ok {
		const loadKeys = true
		fn = m.getParsedCAOnceFn(ctx, id, loadKeys)
		m.cachedGetters[id] = fn
	}
	return fn
}

func (s *Service) caOverrideFromEvent(ctx context.Context, e *types.Event) (_ *subcav1.CertAuthorityOverride, ok bool) {
	return resourceFromEvent[*subcav1.CertAuthorityOverride](ctx, s.logger, e)
}

func (s *Service) pendingCSRRequestFromEvent(ctx context.Context, e *types.Event) (_ *subcav1.PendingCSRRequest, ok bool) {
	return resourceFromEvent[*subcav1.PendingCSRRequest](ctx, s.logger, e)
}

type resourcePtr[T any] interface {
	types.Resource153
	*T
}

func resourceFromEvent[P resourcePtr[T], T any](
	ctx context.Context,
	logger *slog.Logger,
	e *types.Event,
) (_ P, ok bool) {
	rw, ok := e.Resource.(types.Resource153UnwrapperT[P])
	if !ok {
		logger.WarnContext(ctx, "Received unexpected resource wrapper from event",
			"resource_type", log.TypeAttr(e.Resource),
			"kind", e.Resource.GetKind(),
			"sub_kind", e.Resource.GetSubKind(),
			"name", e.Resource.GetName(),
		)
		return nil, false
	}
	resource := rw.UnwrapT()
	if resource == nil {
		logger.WarnContext(ctx, "Received nil resource from event",
			"kind", e.Resource.GetKind(),
			"sub_kind", e.Resource.GetSubKind(),
			"name", e.Resource.GetName(),
		)
		return nil, false
	}
	return resource, true
}
