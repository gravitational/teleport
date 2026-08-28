// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package endpoint

import (
	"context"
	"log/slog"
	"sync/atomic"

	smithyendpoints "github.com/aws/smithy-go/endpoints"
	"github.com/gravitational/trace"
)

// Resolver is a generalized version of an EndpointResolverV2 for
// services in aws-sdk-go-v2. The generic parameter MUST match the
// EndpointParameters of the aws service.
type Resolver[P any] interface {
	ResolveEndpoint(context.Context, P) (smithyendpoints.Endpoint, error)
}

// LoggingResolver is a [Resolver] implementation that logs
// the resolved endpoint of the aws service. It will only
// log the endpoint one the first resolution, and any later
// resolutions in which the endpoint has changed to prevent
// excess log spam.
type LoggingResolver[P any] struct {
	inner  Resolver[P]
	logger *slog.Logger

	last atomic.Pointer[string]
}

// NewLoggingResolver creates a [LoggingResolver] that defers to the
// provided [Resolver] to do the resolution.
func NewLoggingResolver[P any](r Resolver[P], logger *slog.Logger) (*LoggingResolver[P], error) {
	if r == nil {
		return nil, trace.BadParameter("a valid resolver must be provided to the LoggingResolver")
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &LoggingResolver[P]{inner: r, logger: logger}, nil
}

// ResolveEndpoint implements [Resolver].
func (r *LoggingResolver[P]) ResolveEndpoint(ctx context.Context, params P) (smithyendpoints.Endpoint, error) {
	endpoint, err := r.inner.ResolveEndpoint(ctx, params)
	if err != nil {
		return endpoint, err
	}

	uri := endpoint.URI.String()
	last := r.last.Swap(&uri)
	if last == nil || *last != uri {
		r.logger.InfoContext(ctx, "resolved endpoint for aws service", "uri", uri)
	}

	return endpoint, nil
}
