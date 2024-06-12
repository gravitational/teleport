/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// MarshalNamespace marshals the Namespace resource to JSON.
func MarshalNamespace(resource types.Namespace, opts ...MarshalOption) ([]byte, error) {
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return utils.FastMarshal(maybeResetProtoResourceID(cfg.PreserveResourceID, &resource))
}

// UnmarshalNamespace unmarshals the Namespace resource from JSON.
func UnmarshalNamespace(data []byte, opts ...MarshalOption) (*types.Namespace, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing namespace data")
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// always skip schema validation on namespaces unmarshal
	// the namespace is always created by teleport now
	var namespace types.Namespace
	if err := utils.FastUnmarshal(data, &namespace); err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if err := namespace.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.ID != 0 {
		namespace.Metadata.ID = cfg.ID
	}
	if cfg.Revision != "" {
		namespace.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		namespace.Metadata.Expires = &cfg.Expires
	}

	return &namespace, nil
}
