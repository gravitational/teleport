package auth

import (
	"context"

	"github.com/gravitational/teleport/lib/events"
)

type contextKey string

const ContextDelegator contextKey = events.AccessRequestDelegator

// WithDelegator creates a child context with the AccessRequestDelegator
// value set.  Optionally used by AuthServer.SetAccessRequestState to log
// a delegating identity.
func WithDelegator(ctx context.Context, delegator string) context.Context {
	return context.WithValue(ctx, ContextDelegator, delegator)
}

// getDelegator attempts to load the context value AccessRequestDelegator,
// returning the empty string if no value was found.
func getDelegator(ctx context.Context) string {
	delegator, ok := ctx.Value(ContextDelegator).(string)
	if !ok {
		return ""
	}
	return delegator
}
