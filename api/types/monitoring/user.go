package monitoring

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
)

// User is the user resource.
type User struct {
	// ResourceHeader is the resource header.
	header.ResourceHeader
	Spec UserSpec `json:"spec" yaml:"spec"`
}

type UserSpec struct {
	// LastUsed is the last time the user was used.
	LastActivity time.Time `json:"last_activity,omitempty" yaml:"last_activity,omitempty"`
	// RolesLastActive is the last time the user used a role.
	RolesLastActive map[string]time.Time `json:"roles_last_active,omitempty" yaml:"roles_last_active,omitempty"`
	LastLogin       time.Time            `json:"last_login,omitempty" yaml:"last_login,omitempty"`
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *User) CheckAndSetDefaults() error {
	a.SetKind(types.KindMonitoringUserState)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewUser ...
func NewUser(metadata header.Metadata, spec UserSpec) (*User, error) {
	out := &User{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *User) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}
