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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services/local"
)

const (
	// watcherBaseDuration is used as both a jittered First retry duration and as
	// the exponential driver step.
	watcherBaseDuration = 5 * time.Second // Arbitrary. Approx MaxWatcherBackoff/16.
)

func (s *Service) condUpdateCAOverride(
	ctx context.Context,
	id local.CertAuthorityOverrideID,
	initial *subcav1.CertAuthorityOverride,
	modify func(*subcav1.CertAuthorityOverride) (done bool, _ error),
) (*subcav1.CertAuthorityOverride, error) {
	updated, err := condUpdateWithRetry(
		ctx,
		s.clock,
		s.subCA.GetCertAuthorityOverride,
		s.subCA.UpdateCertAuthorityOverride,
		id,
		initial,
		modify,
	)
	return updated, trace.Wrap(err)
}

func (s *Service) condUpdatePendingCSRRequest(
	ctx context.Context,
	name string,
	initial *subcav1.PendingCSRRequest,
	modify func(*subcav1.PendingCSRRequest) (done bool, _ error),
) (*subcav1.PendingCSRRequest, error) {
	updated, err := condUpdateWithRetry(
		ctx,
		s.clock,
		s.pendingCSR.GetPendingCSRRequest,
		s.pendingCSR.UpdatePendingCSRRequest,
		name,
		initial,
		modify,
	)
	return updated, trace.Wrap(err)
}

func condUpdateWithRetry[ID any, P *T, T any](
	ctx context.Context,
	clock clockwork.Clock,
	getter func(context.Context, ID) (P, error),
	updater func(context.Context, P) (P, error),
	id ID,
	initial P,
	modify func(P) (done bool, _ error),
) (P, error) {
	var updated P
	const maxAttempts = 5 // Arbitrary.
	err := retryutils.UpdateWithRetry(
		ctx,
		clock,
		// Refresher.
		func(ctx context.Context, isRetry bool) (P, error) {
			if !isRetry && initial != nil {
				return initial, nil
			}
			resource, err := getter(ctx, id)
			return resource, trace.Wrap(err, "read resource")
		},
		// Updater.
		func(ctx context.Context, resource P) error {
			switch done, err := modify(resource); {
			case done:
				return nil
			case err != nil:
				return trace.Wrap(err)
			}

			var err error
			updated, err = updater(ctx, resource)
			return trace.Wrap(err)
		},
		retryutils.WithMaxRetries(maxAttempts),
	)
	return updated, trace.Wrap(err)
}

func (s *Service) newWatcherRetrier() (*retryutils.RetryV2, error) {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		First:  retryutils.FullJitter(watcherBaseDuration),
		Driver: retryutils.NewExponentialDriver(watcherBaseDuration),
		Max:    defaults.MaxWatcherBackoff,
		Jitter: retryutils.HalfJitter,
		Clock:  s.clock,
	})
	return retry, trace.Wrap(err)
}
