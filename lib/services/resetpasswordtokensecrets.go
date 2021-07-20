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

	"github.com/gravitational/teleport/api/v7/types"
	"github.com/gravitational/teleport/lib/utils"
)

// UnmarshalResetPasswordTokenSecrets unmarshals the ResetPasswordTokenSecrets resource from JSON.
func UnmarshalResetPasswordTokenSecrets(bytes []byte, opts ...MarshalOption) (types.ResetPasswordTokenSecrets, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var secrets types.ResetPasswordTokenSecretsV3
	if err := utils.FastUnmarshal(bytes, &secrets); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &secrets, nil
}

// MarshalResetPasswordTokenSecrets marshals the ResetPasswordTokenSecrets resource to JSON.
func MarshalResetPasswordTokenSecrets(secrets types.ResetPasswordTokenSecrets, opts ...MarshalOption) ([]byte, error) {
	if err := secrets.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return utils.FastMarshal(secrets)
}
