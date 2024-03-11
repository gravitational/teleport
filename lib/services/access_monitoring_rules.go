/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/accessmonitoringrules"
	"github.com/gravitational/teleport/api/types/accessmonitoringrule"
	"github.com/gravitational/teleport/lib/utils"
)

var _ AccessMonitoringRules = (*accessmonitoringrules.Client)(nil)

// AccessMonitoringRules is the AccessMonitoringRule service
type AccessMonitoringRules interface {
	CreateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error)
	UpdateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error)
	UpsertAccessMonitoringRule(ctx context.Context, in *accessmonitoringrule.AccessMonitoringRule) (*accessmonitoringrule.AccessMonitoringRule, error)
	GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrule.AccessMonitoringRule, error)
	DeleteAccessMonitoringRule(ctx context.Context, name string) error
	DeleteAllAccessMonitoringRules(ctx context.Context) error
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrule.AccessMonitoringRule, string, error)
}

// MarshalAccessMonitoringRule marshals AccessMonitoringRule resource to JSON.
func MarshalAccessMonitoringRule(accessMonitoringRule *accessmonitoringrule.AccessMonitoringRule, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.PreserveResourceID {
		copy := *accessMonitoringRule
		copy.Metadata.ID = 0
		copy.Metadata.Revision = ""
		accessMonitoringRule = &copy
	}
	return utils.FastMarshal(accessMonitoringRule)
}

// UnmarshalAccessMonitoringRule unmarshals the AccessMonitoringRule resource.
func UnmarshalAccessMonitoringRule(data []byte, opts ...MarshalOption) (*accessmonitoringrule.AccessMonitoringRule, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access monitoring rule data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var accessMonitoringRule accessmonitoringrule.AccessMonitoringRule
	if err := utils.FastUnmarshal(data, &accessMonitoringRule); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if cfg.ID != 0 {
		accessMonitoringRule.Metadata.ID = cfg.ID
	}
	if cfg.Revision != "" {
		accessMonitoringRule.Metadata.Revision =  cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		accessMonitoringRule.Metadata.Expires = cfg.Expires
	}
	return &accessMonitoringRule, nil
}
