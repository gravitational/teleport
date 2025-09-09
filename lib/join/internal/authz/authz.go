package authz

import (
	"github.com/gravitational/teleport/api/types"
)

// Context represents a distilled authenticated context for a join request.
type Context struct {
	// IsForwardedByProxy is true if the join request was actively forwarded by
	// a proxy. As in, the proxy terminated the gRPC request, added
	// proxy-supplied parameters, and forwarded it to the auth service.
	IsForwardedByProxy bool
	// IsInstance is true if the client authenticated as the Instance system role.
	IsInstance bool
	// IsBot is true if the client authenticated as the Bot system role.
	IsBot bool
	// SystemRoles is a list of additional system roles that an Instance
	// authenticated as having.
	SystemRoles types.SystemRoles
	// HostID is an authenticated HostID.
	HostID string
	// BotGeneration is the current generation of an authenticated Bot.
	BotGeneration uint64
	// BotInstanceID is an authenticted Bot ID.
	BotInstanceID string
}
