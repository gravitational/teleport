/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package services

import (
	"github.com/gravitational/trace"

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
	if err := license.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch license := license.(type) {
	case *types.LicenseV3:
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
