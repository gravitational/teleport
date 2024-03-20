package devicetrustv1_test

import (
	"context"
	"slices"
	"sync"

	"google.golang.org/grpc/metadata"

	apievents "github.com/gravitational/teleport/api/types/events"
)

const keyedEmitterKey = "keyedemitter.key"

// withOutgoingEmitterKey assigns a [keyedEmitter] key to an outgoing context.
func withOutgoingEmitterKey(ctx context.Context, key string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, keyedEmitterKey, key)
}

// keyedEmitter is an [apievents.Emitter] that assigns events to user-supplied
// keys.
//
// See [withOutgoingEmitterKey].
type keyedEmitter struct {
	mu     sync.Mutex
	events map[string][]apievents.AuditEvent // keyed by ctx key
}

func (e *keyedEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	var key string

	// Find the ctx key. If absent we record the events against the empty string.
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md[keyedEmitterKey]; len(vals) > 0 {
			key = vals[0]
		}
	}

	e.mu.Lock()
	if e.events == nil {
		e.events = make(map[string][]apievents.AuditEvent)
	}
	e.events[key] = append(e.events[key], event)
	e.mu.Unlock()

	return nil
}

func (e *keyedEmitter) Events(key string) []apievents.AuditEvent {
	e.mu.Lock()
	val := e.events[key]
	e.mu.Unlock()

	return slices.Clone(val)
}

func (e *keyedEmitter) LastEvent(key string) apievents.AuditEvent {
	e.mu.Lock()
	val := e.events[key]
	e.mu.Unlock()

	if l := len(val); l > 0 {
		return val[l-1]
	}
	return nil
}
