/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
