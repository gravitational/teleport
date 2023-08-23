// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils"
)

func (d *MFADevice) WithoutSensitiveData() (*MFADevice, error) {
	if d == nil {
		return nil, trace.BadParameter("cannot hide sensitive data on empty object")
	}
	out := utils.CloneProtoMsg(d)

	switch mfad := out.Device.(type) {
	case *MFADevice_Totp:
		mfad.Totp.Key = ""
	case *MFADevice_U2F:
		// OK, no sensitive secrets.
	case *MFADevice_Webauthn:
		// OK, no sensitive secrets.
	default:
		return nil, trace.BadParameter("unsupported MFADevice type %T", d.Device)
	}

	return out, nil
}
