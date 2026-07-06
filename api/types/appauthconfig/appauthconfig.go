// Copyright 2026 Gravitational, Inc.
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

package appauthconfig

import (
	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewAppAuthConfigJWT creates a new app auth configuration resource for JWT.
func NewAppAuthConfigJWT(name string, labels []*labelv1.Label, spec *appauthconfigv1.AppAuthConfigJWTSpec) *appauthconfigv1.AppAuthConfig {
	return &appauthconfigv1.AppAuthConfig{
		Kind:    types.KindAppAuthConfig,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &appauthconfigv1.AppAuthConfigSpec{
			AppLabels: labels,
			SubKindSpec: &appauthconfigv1.AppAuthConfigSpec_Jwt{
				Jwt: spec,
			},
		},
	}
}
