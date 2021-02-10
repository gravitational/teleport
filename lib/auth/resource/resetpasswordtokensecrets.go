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

package resource

import (
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// ResetPasswordTokenSecretsSpecV3Template is a template for V3 ResetPasswordTokenSecrets JSON schema
const ResetPasswordTokenSecretsSpecV3Template = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "opt_key": {
		"type": ["string"]
	  },
	  "qr_code": {
		"type": ["string"]
	  },
	  "created": {
		"type": ["string"]
	  }
	}
  }`

// UnmarshalResetPasswordTokenSecrets unmarshals the ResetPasswordTokenSecrets resource from JSON.
func UnmarshalResetPasswordTokenSecrets(bytes []byte, opts ...auth.MarshalOption) (ResetPasswordTokenSecrets, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var secrets ResetPasswordTokenSecretsV3
	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ResetPasswordTokenSecretsSpecV3Template, DefaultDefinitions)
	err := utils.UnmarshalWithSchema(schema, &secrets, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &secrets, nil
}

// MarshalResetPasswordTokenSecrets marshals the ResetPasswordTokenSecrets resource to JSON.
func MarshalResetPasswordTokenSecrets(secrets ResetPasswordTokenSecrets, opts ...auth.MarshalOption) ([]byte, error) {
	return utils.FastMarshal(secrets)
}
