package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

type AccessCheckerMonitor struct {
	AccessChecker
	roleUpdater roleUpdater
}

func (m *AccessCheckerMonitor) GetUsedRoles() []string {
	var out []string
	for _, v := range m.Roles() {
		r, ok := v.(*MonitorRole)
		if !ok {
			continue
		}
		if r.GetUsed() {
			out = append(out, r.GetName())
		}
	}
	return out
}

type roleUpdater interface {
	UpdateRole(ctx context.Context, role types.Role) (types.Role, error)
}

type RoleStore struct {
	Roles []string
}

func (r *RoleStore) UpdateRole(ctx context.Context, role types.Role) (types.Role, error) {
	r.Roles = append(r.Roles, role.GetName())
	return role, nil
}

func NewAccessMonitor(inner AccessChecker, updater roleUpdater) AccessChecker {
	return &AccessCheckerMonitor{
		AccessChecker: inner,
		roleUpdater:   updater,
	}
}

func NewAccessMonitor2(inner AccessChecker) (*AccessCheckerMonitor, error) {
	ac, ok := inner.(*accessChecker)
	if !ok {
		return nil, trace.BadParameter("invalid type")
	}
	ac.RoleSet = toMonitorRole(ac.Roles())
	return &AccessCheckerMonitor{
		AccessChecker: ac,
	}, nil
}

func toMonitorRole(rs RoleSet) RoleSet {
	var out RoleSet
	for _, v := range rs {
		out = append(out, &MonitorRole{Role: v})
	}
	return out
}

type MonitorRole struct {
	types.Role
	used bool
}

type roleUsedSetter interface {
	SetUsed()
	GetUsed() bool
}

func (m *MonitorRole) SetUsed() {
	m.used = true
}

func (m *MonitorRole) GetUsed() bool {
	return m.used
}

func setRoleAsUsed(r types.Role) {
	if v, ok := r.(roleUsedSetter); ok {
		v.SetUsed()
	}
}
