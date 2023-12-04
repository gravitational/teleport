package services

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/monitoring"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalMonitoringUser marshals an audit query.
func MarshalMonitoringUser(in *monitoring.User, opts ...MarshalOption) ([]byte, error) {
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		tmp := *in
		tmp.SetResourceID(0)
		in = &tmp
	}
	return utils.FastMarshal(in)
}

// UnmarshalMonitoringUser unmarshals an audit query.
func UnmarshalMonitoringUser(data []byte, opts ...MarshalOption) (*monitoring.User, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *monitoring.User
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}

// MarshalMonitoringRole marshals an audit query.
func MarshalMonitoringRole(in *monitoring.Role, opts ...MarshalOption) ([]byte, error) {
	if err := in.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		tmp := *in
		tmp.SetResourceID(0)
		in = &tmp
	}
	return utils.FastMarshal(in)
}

// UnmarshalMonitoringRole unmarshals an audit query.
func UnmarshalMonitoringRole(data []byte, opts ...MarshalOption) (*monitoring.Role, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access list data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out *monitoring.Role
	if err := utils.FastUnmarshal(data, &out); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.ID != 0 {
		out.SetResourceID(cfg.ID)
	}
	if !cfg.Expires.IsZero() {
		out.SetExpiry(cfg.Expires)
	}
	return out, nil
}
