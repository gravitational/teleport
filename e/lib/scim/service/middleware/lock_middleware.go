package middleware

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// LockMiddleware wraps a ResourceHandler to provide distributed locking
// for PATCH operations on SCIM resources. This prevents race conditions
// when multiple concurrent PATCH requests modify the same resource.
type LockMiddleware struct {
	LockMiddlewareConfig
	ForwardingMiddleware
}

// LockMiddlewareConfig contains the configuration for the lock middleware.
type LockMiddlewareConfig struct {
	// Locker provides distributed lock acquisition/release functionality.
	Locker common.Locker
	// Log is the logger for recording lock-related events and errors.
	Log *slog.Logger
	// PluginName is the name of the SCIM plugin, used as part of the lock key.
	PluginName string
}

// checkAndSetDefaults validates the middleware configuration and sets default values.
func (l *LockMiddlewareConfig) checkAndSetDefaults() error {
	if l.Locker == nil {
		return trace.BadParameter("locker is required")
	}
	if l.PluginName == "" {
		return trace.BadParameter("plugin name is required")
	}
	if l.Log == nil {
		l.Log = slog.Default()
	}
	return nil
}

// NewLockMiddleware creates a new lock middleware with the given configuration.
// The middleware will wrap the provided ResourceHandler to add distributed locking
// for PATCH operations, preventing concurrent modifications to the same resource.
func NewLockMiddleware(config LockMiddlewareConfig) (*LockMiddleware, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &LockMiddleware{
		LockMiddlewareConfig: config,
	}, nil
}

// PatchResourceMiddleware handles PATCH operations on SCIM resources with distributed locking.
// It acquires a lock based on the plugin name and resource ID to prevent concurrent
// modifications to the same resource, which could lead to race conditions.
//
// The lock is automatically released when the function returns, even if an error occurs.
func (l *LockMiddleware) PatchResourceMiddleware(ctx context.Context, req *pb.PatchSCIMResourceRequest, next common.ResourceHandler) (*pb.Resource, error) {
	if req == nil {
		return nil, trace.BadParameter("request cannot be nil")
	}
	if req.GetTarget() == nil {
		return nil, trace.BadParameter("request target cannot be nil")
	}
	resourceID := req.GetTarget().GetResourceId()
	if resourceID == "" {
		return nil, trace.BadParameter("resource ID cannot be empty")
	}

	// Example: "my-plugin:patch:resource-123"
	lockKey := fmt.Sprintf("%s:patch:%s", l.PluginName, resourceID)

	unlock, err := l.Locker.Acquire(ctx, lockKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to acquire lock for resource %q", resourceID)
	}

	defer func() {
		if err := unlock(ctx); err != nil {
			l.Log.DebugContext(ctx, "Failed to release lock", "error", err, "resource_id", resourceID)
		}
	}()

	// Call the next handler in the chain
	resp, err := next.PatchResource(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}
