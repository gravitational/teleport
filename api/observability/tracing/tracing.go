// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// PropagationContext contains tracing information to be passed across service boundaries
type PropagationContext map[string]string

// PropagationContextFromContext creates a PropagationContext from the given context.Context. If the context
// does not contain any tracing information, the PropagationContext will be empty.
func PropagationContextFromContext(ctx context.Context) PropagationContext {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, &carrier)
	return PropagationContext(carrier)
}

// WithPropagationContext injects any tracing information from the given PropagationContext into the
// given context.Context.
func WithPropagationContext(ctx context.Context, pc PropagationContext) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(pc))
}
