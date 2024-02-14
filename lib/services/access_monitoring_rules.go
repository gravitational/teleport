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
	"bytes"
	"context"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// AccessMonitoringRules is the AccessMonitoringRule service
type AccessMonitoringRules interface {
	CreateAccessMonitoringRule(ctx context.Context, in types.AccessMonitoringRule) (types.AccessMonitoringRule, error)
	UpdateAccessMonitoringRule(ctx context.Context, in types.AccessMonitoringRule) (types.AccessMonitoringRule, error)
	UpsertAccessMonitoringRule(ctx context.Context, in types.AccessMonitoringRule) (types.AccessMonitoringRule, error)
	GetAccessMonitoringRule(ctx context.Context, name string) (types.AccessMonitoringRule, error)
	DeleteAccessMonitoringRule(ctx context.Context, name string) error
	ListAccessMonitoringRules(ctx context.Context, limit int, startKey string) ([]types.AccessMonitoringRule, string, error)
}

// MarshalAccessMonitoringRule marshals AccessMonitoringRule resource to JSON.
func MarshalAccessMonitoringRule(accessMonitoringRule types.AccessMonitoringRule, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch AccessMonitoringRule := accessMonitoringRule.(type) {
	case *types.AccessMonitoringRuleV1:
		if err := AccessMonitoringRule.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		var buf bytes.Buffer
		err := (&jsonpb.Marshaler{}).Marshal(&buf, maybeResetProtoResourceID(cfg.PreserveResourceID, AccessMonitoringRule))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return buf.Bytes(), nil
	default:
		return nil, trace.BadParameter("unsupported AccessMonitoringRule resource %T", AccessMonitoringRule)
	}
}

// UnmarshalAccessMonitoringRule unmarshals the AccessMonitoringRule resource.
func UnmarshalAccessMonitoringRule(data []byte, opts ...MarshalOption) (types.AccessMonitoringRule, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing AccessMonitoringRule resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V1:
		var accessMonitoringRule types.AccessMonitoringRuleV1
		if err := utils.FastUnmarshal(data, &accessMonitoringRule); err != nil {
			return nil, trace.BadParameter(err.Error())
		}

		if err := accessMonitoringRule.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			accessMonitoringRule.SetResourceID(cfg.ID)
		}
		if cfg.Revision != "" {
			accessMonitoringRule.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			accessMonitoringRule.SetExpiry(cfg.Expires)
		}
		return &accessMonitoringRule, nil
	}
	return nil, trace.BadParameter("unsupported AccessMonitoringRule resource version %q", h.Version)
}
