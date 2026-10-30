// Teleport
// Copyright (C) 2026  Gravitational, Inc.
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

package auth

import (
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	delegatedAccessRequestRatePeriod = time.Minute
	delegatedAccessRequestRateBurst  = 10
)

type delegatedAccessRequestLimiter struct {
	limiter *limiter.RateLimiter
}

func newDelegatedAccessRequestLimiter(clock clockwork.Clock) (*delegatedAccessRequestLimiter, error) {
	rateLimiter, err := limiter.NewRateLimiter(limiter.Config{
		Clock: clock,
		Rates: []limiter.Rate{{
			Period:  delegatedAccessRequestRatePeriod,
			Average: 1,
			Burst:   delegatedAccessRequestRateBurst,
		}},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &delegatedAccessRequestLimiter{limiter: rateLimiter}, nil
}

func (l *delegatedAccessRequestLimiter) check(identity tlsca.Identity) error {
	if len(identity.Delegation.GetChain()) == 0 {
		return nil
	}

	// Include the origin cluster so identities from different clusters do not
	// share a bucket if their usernames happen to be the same.
	key := identity.OriginClusterName + "\x00" + identity.Username
	return trace.Wrap(l.limiter.RegisterRequest(key))
}
