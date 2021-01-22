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
	"fmt"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// LicenseSpecV3Template is a template for V3 License JSON schema
const LicenseSpecV3Template = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
		"account_id": {
			"type": ["string"]
		},
		"plan_id": {
			"type": ["string"]
		},
		"usage": {
			"type": ["string", "boolean"]
		},
		"aws_pid": {
			"type": ["string"]
		},
		"aws_account": {
			"type": ["string"]
		},
		"k8s": {
			"type": ["string", "boolean"]
		},
		"cloud": {
			"type": ["string", "boolean"]
		}
  }
}`

// UnmarshalLicense unmarshals the License resource.
// and validates schema
func UnmarshalLicense(bytes []byte) (License, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, LicenseSpecV3Template, DefaultDefinitions)

	var license LicenseV3
	err := utils.UnmarshalWithSchema(schema, &license, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if license.Version != V3 {
		return nil, trace.BadParameter("unsupported version %v, expected version %v", license.Version, V3)
	}

	if err := license.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &license, nil
}

// MarshalLicense marshals the License resource.
func MarshalLicense(license License, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch resource := license.(type) {
	case *LicenseV3:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *resource
			copy.SetResourceID(0)
			resource = &copy
		}
		return utils.FastMarshal(resource)
	default:
		return nil, trace.BadParameter("unrecognized resource version %T", license)
	}
}
