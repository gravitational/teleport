package proto

import "github.com/gravitational/trace"

// CheckAndSetDefaults checks and sets default values
func (req *GenerateServerKeysRequest) CheckAndSetDefaults() error {
	if req.HostID == "" {
		return trace.BadParameter("missing parameter HostID")
	}
	if len(req.Roles) != 1 {
		return trace.BadParameter("expected only one system role, got %v", len(req.Roles))
	}
	return nil
}
