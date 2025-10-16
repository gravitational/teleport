package provision

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// A Token is used in the join service to facilitate provisioning.
type Token interface {
	// GetName returns the name of the token.
	GetName() string
	// GetSafeName returns the name of the token, sanitized appropriately for
	// join methods where the name is secret. This should be used when logging
	// the token name.
	GetSafeName() string
	// GetJoinMethod returns joining method that must be used with this token.
	GetJoinMethod() types.JoinMethod
	// GetRoles returns a list of teleport roles that will be granted to the
	// resources provisioned with this token.
	GetRoles() types.SystemRoles
	// Expiry returns the token's expiration time.
	Expiry() time.Time
	// GetBotName returns the BotName field which must be set for joining bots.
	GetBotName() string
	// GetAssignedScope returns the scope that will be assigned to provisioned resources
	// provisioned using the wrapped [joiningv1.ScopedToken].
	GetAssignedScope() string
}
