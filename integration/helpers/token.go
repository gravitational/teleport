package helpers

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

func MustCreateProvisionToken(token string, roles types.SystemRoles, expires time.Time) types.ProvisionToken {
	t, err := types.NewProvisionToken(token, roles, expires)
	if err != nil {
		panic(err)
	}
	return t
}
