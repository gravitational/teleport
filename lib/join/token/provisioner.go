package token

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

type Provisioner interface {
	GetName() string
	GetSafeName() string
	GetJoinMethod() types.JoinMethod
	GetRoles() types.SystemRoles
	Expiry() time.Time
	GetBotName() string
	GetAssignedScope() string
}
