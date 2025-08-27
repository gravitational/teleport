/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package events

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"

	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// SearchEventsLimiter allows to wrap any AuditLogger with rate limit on
// search events endpoints.
// Note it share limiter for both SearchEvents and SearchSessionEvents.
type SearchEventsLimiter struct {
	limiter *rate.Limiter
	AuditLogger
}

// SearchEventsLimiterConfig is configuration for SearchEventsLimiter.
type SearchEventsLimiterConfig struct {
	// RefillTime determines the duration of time between the addition of tokens to the bucket.
	RefillTime time.Duration
	// RefillAmount is the number of tokens that are added to the bucket during interval
	// specified by RefillTime.
	RefillAmount int
	// Burst defines number of available tokens. It's initially full and refilled
	// based on RefillAmount and RefillTime.
	Burst int
	// AuditLogger is auditLogger that will be wrapped with limiter on search endpoints.
	AuditLogger AuditLogger
}

func (cfg *SearchEventsLimiterConfig) CheckAndSetDefaults() error {
	if cfg.AuditLogger == nil {
		return trace.BadParameter("empty auditLogger")
	}
	if cfg.Burst <= 0 {
		return trace.BadParameter("Burst cannot be less or equal to 0")
	}
	if cfg.RefillAmount <= 0 {
		return trace.BadParameter("RefillAmount cannot be less or equal to 0")
	}
	if cfg.RefillTime == 0 {
		// Default to seconds so it can be just used as rate.
		cfg.RefillTime = time.Second
	}
	return nil
}

// NewSearchEventLimiter returns instance of new SearchEventsLimiter.
func NewSearchEventLimiter(cfg SearchEventsLimiterConfig) (*SearchEventsLimiter, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &SearchEventsLimiter{
		limiter:     rate.NewLimiter(rate.Every(cfg.RefillTime/time.Duration(cfg.RefillAmount)), cfg.Burst),
		AuditLogger: cfg.AuditLogger,
	}, nil
}

func (s *SearchEventsLimiter) SearchUnstructuredEvents(ctx context.Context, req SearchEventsRequest) ([]*auditlogpb.EventUnstructured, string, error) {
	if !s.limiter.Allow() {
		return nil, "", trace.LimitExceeded("rate limit exceeded for searching events")
	}
	out, keyset, err := s.AuditLogger.SearchUnstructuredEvents(ctx, req)
	return out, keyset, trace.Wrap(err)
}

func (s *SearchEventsLimiter) SearchEvents(ctx context.Context, req SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	if !s.limiter.Allow() {
		return nil, "", trace.LimitExceeded("rate limit exceeded for searching events")
	}
	out, keyset, err := s.AuditLogger.SearchEvents(ctx, req)
	return out, keyset, trace.Wrap(err)
}

func (s *SearchEventsLimiter) SearchSessionEvents(ctx context.Context, req SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	if !s.limiter.Allow() {
		return nil, "", trace.LimitExceeded("rate limit exceeded for searching events")
	}
	out, keyset, err := s.AuditLogger.SearchSessionEvents(ctx, req)
	return out, keyset, trace.Wrap(err)
}
