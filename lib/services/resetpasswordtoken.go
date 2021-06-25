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

// UnmarshalResetPasswordToken unmarshals the ResetPasswordToken resource from JSON.
func UnmarshalResetPasswordToken(bytes []byte, opts ...MarshalOption) (types.ResetPasswordToken, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var token types.ResetPasswordTokenV3
	if err := utils.FastUnmarshal(bytes, &token); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := token.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &token, nil
}

// MarshalResetPasswordToken marshals the ResetPasswordToken resource to JSON.
func MarshalResetPasswordToken(token types.ResetPasswordToken, opts ...MarshalOption) ([]byte, error) {
	if err := token.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return utils.FastMarshal(token)
}
