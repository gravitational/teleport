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
	"google.golang.org/protobuf/types/known/timestamppb"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessMonitoringRules is the AccessMonitoringRule service
type AccessMonitoringRules interface {
	CreateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	UpdateAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	UpsertAccessMonitoringRule(ctx context.Context, in *accessmonitoringrulesv1.AccessMonitoringRule) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	GetAccessMonitoringRule(ctx context.Context, name string) (*accessmonitoringrulesv1.AccessMonitoringRule, error)
	DeleteAccessMonitoringRule(ctx context.Context, name string) error
	DeleteAllAccessMonitoringRules(ctx context.Context) error
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}

// MarshalAccessMonitoringRule marshals AccessMonitoringRule resource to JSON.
func MarshalAccessMonitoringRule(accessMonitoringRule *accessmonitoringrulesv1.AccessMonitoringRule, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(accessMonitoringRule)
}

// UnmarshalAccessMonitoringRule unmarshals the AccessMonitoringRule resource.
func UnmarshalAccessMonitoringRule(data []byte, opts ...MarshalOption) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing access monitoring rule data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var accessMonitoringRule accessmonitoringrulesv1.AccessMonitoringRule
	if err := utils.FastUnmarshal(data, &accessMonitoringRule); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if cfg.Revision != "" {
		accessMonitoringRule.Metadata.Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		accessMonitoringRule.Metadata.Expires = timestamppb.New(cfg.Expires)
	}
	return &accessMonitoringRule, nil
}
