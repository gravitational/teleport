// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package typestest

import (
	fmt "fmt"
	"net/url"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

var KindApp string = "application"
var V3 string = "v3"

// setStaticFields sets static resource header and metadata fields.
func (a *AppV3) setStaticFields() {
	a.Kind = KindApp
	a.Version = V3
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (a *AppV3) CheckAndSetDefaults() error {
	a.setStaticFields()
	if err := a.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	for key := range a.Spec.DynamicLabels {
		if !IsValidLabelKey(key) {
			return trace.BadParameter("app %q invalid label key: %q", a.GetName(), key)
		}
	}
	if a.Spec.URI == "" {
		if a.Spec.Cloud != "" {
			a.Spec.URI = fmt.Sprintf("cloud://%v", a.Spec.Cloud)
		} else {
			return trace.BadParameter("app %q URI is empty", a.GetName())
		}
	}
	if a.Spec.Cloud == "" && a.IsAWSConsole() {
		a.Spec.Cloud = CloudAWS
	}
	switch a.Spec.Cloud {
	case "", CloudAWS, CloudAzure, CloudGCP:
		break
	default:
		return trace.BadParameter("app %q has unexpected Cloud value %q", a.GetName(), a.Spec.Cloud)
	}
	url, err := url.Parse(a.Spec.PublicAddr)
	if err != nil {
		return trace.BadParameter("invalid PublicAddr format: %v", err)
	}
	host := a.Spec.PublicAddr
	if url.Host != "" {
		host = url.Host
	}

	if strings.HasPrefix(host, constants.KubeTeleportProxyALPNPrefix) {
		return trace.BadParameter("app %q DNS prefix found in %q public_url is reserved for internal usage",
			constants.KubeTeleportProxyALPNPrefix, a.Spec.PublicAddr)
	}

	if a.Spec.Rewrite != nil {
		switch a.Spec.Rewrite.JWTClaims {
		case "", JWTClaimsRewriteRolesAndTraits, JWTClaimsRewriteRoles, JWTClaimsRewriteNone:
		default:
			return trace.BadParameter("app %q has unexpected JWT rewrite value %q", a.GetName(), a.Spec.Rewrite.JWTClaims)

		}
	}

	return nil
}
