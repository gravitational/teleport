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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalLicense unmarshals the License resource from JSON.
func UnmarshalLicense(bytes []byte) (types.License, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var license types.LicenseV3
	err := utils.FastUnmarshal(bytes, &license)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if license.Version != types.V3 {
		return nil, trace.BadParameter("unsupported version %v, expected version %v", license.Version, types.V3)
	}

	if err := license.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &license, nil
}

// MarshalLicense marshals the License resource to JSON.
func MarshalLicense(license types.License, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch license := license.(type) {
	case *types.LicenseV3:
		if err := license.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *license
			copy.SetResourceID(0)
			copy.SetRevision("")
			license = &copy
		}
		return utils.FastMarshal(license)
	default:
		return nil, trace.BadParameter("unrecognized license version %T", license)
	}
}

// IsDashboard returns a bool indicating if the cluster is a
// dashboard cluster.
// Dashboard is a cluster running on cloud infrastructure that
// isn't a Teleport Cloud cluster
func IsDashboard(features proto.Features) bool {
	// TODO(matheus): for now, we assume dashboard based on
	// the presence of recovery codes, which are never enabled
	// in OSS or self-hosted Teleport.
	return !features.GetCloud() && features.GetRecoveryCodes()
}
