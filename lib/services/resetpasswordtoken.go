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

// ResetPasswordTokenSpecV3Template is a template for V3 ResetPasswordToken JSON schema
const ResetPasswordTokenSpecV3Template = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
	  "user": {
		"type": ["string"]
	  },
	  "created": {
		"type": ["string"]
	  },
	  "url": {
		"type": ["string"]
	  }
	}
  }`

// UnmarshalResetPasswordToken unmarshals the ResetPasswordToken resource.
func UnmarshalResetPasswordToken(bytes []byte, opts ...MarshalOption) (ResetPasswordToken, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var token ResetPasswordTokenV3
	schema := fmt.Sprintf(V2SchemaTemplate, MetadataSchema, ResetPasswordTokenSpecV3Template, DefaultDefinitions)
	err := utils.UnmarshalWithSchema(schema, &token, bytes)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	return &token, nil
}

// MarshalResetPasswordToken marshals the ResetPasswordToken resource.
func MarshalResetPasswordToken(token ResetPasswordToken, opts ...MarshalOption) ([]byte, error) {
	return utils.FastMarshal(token)
}
