package monitoring

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/header/convert/legacy"
)

// Role is the role resource.
type Role struct {
	// ResourceHeader is the resource header.
	header.ResourceHeader
	Spec RoleSpec `json:"spec" yaml:"spec"`
}

type RoleSpec struct {
	LastUsed         time.Time         `json:"last_used,omitempty" yaml:"last_used,omitempty"`
	IsActive         bool              `json:"is_active,omitempty" yaml:"is_active,omitempty"`
	Name             string            `json:"name,omitempty" yaml:"name,omitempty"`
	LoginsLastActive map[string]string `json:"logins_last_active,omitempty" yaml:"logins_last_active,omitempty"`
}

// CheckAndSetDefaults validates fields and populates empty fields with default values.
func (a *Role) CheckAndSetDefaults() error {
	a.SetKind(types.KindMonitoringRoleState)
	a.SetVersion(types.V1)

	if err := a.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetMetadata returns metadata. This is specifically for conforming to the Resource interface,
// and should be removed when possible.
func (a *Role) GetMetadata() types.Metadata {
	return legacy.FromHeaderMetadata(a.Metadata)
}

// NewRole ...
func NewRole(metadata header.Metadata, spec RoleSpec) (*Role, error) {
	secReport := &Role{
		ResourceHeader: header.ResourceHeaderFromMetadata(metadata),
		Spec:           spec,
	}
	if err := secReport.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return secReport, nil
}
