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

package joinuri

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/onboarding"
)

const (
	// URISchemePrefix is the prefix for
	URISchemePrefix = "tbot"

	// BoundKeypairSafeName is a URL scheme-safe name for the bound_keypair join
	// method.
	BoundKeypairSafeName = "bound-keypair"

	// AzureDevopsSafeName is a URL scheme-safe name for the azure_devops join
	// method.
	AzureDevopsSafeName = "azure-devops"

	// TerraformCloudSafeName is a URL scheme-safe name for the terraform_cloud
	// join method.
	TerraformCloudSafeName = "terraform-cloud"
)

type JoinURI struct {
	// AddressKind is the type of joining address, i.e. proxy or auth.
	AddressKind connection.AddressKind

	// JoinMethod is the join method to use when joining, in combination with
	// the token name.
	JoinMethod types.JoinMethod

	// Token is the token name to use when joining
	Token string

	// JoinMethodParameter is an optional parameter to pass to the join method.
	// Its specific meaning depends on the join method in use.
	JoinMethodParameter string

	// Address is either an auth or proxy address, depending on the configured
	// AddressKind. It includes the port.
	Address string
}

func (u *JoinURI) ToURL() *url.URL {
	// Assume "proxy"
	kind := string(connection.AddressKindProxy)
	if u.AddressKind == connection.AddressKindAuth {
		kind = string(connection.AddressKindAuth)
	}

	method := MapJoinMethodToURLSafe(u.JoinMethod)

	var info *url.Userinfo
	if u.JoinMethodParameter != "" {
		info = url.UserPassword(u.Token, u.JoinMethodParameter)
	} else {
		info = url.User(u.Token)
	}

	return &url.URL{
		Scheme: fmt.Sprintf("%s+%s+%s", URISchemePrefix, kind, method),
		User:   info,
		Host:   u.Address,
	}
}

func (u *JoinURI) String() string {
	return u.ToURL().String()
}

// MapURLSafeJoinMethod converts a URL safe join method name to a defined join
// method constant.
func MapURLSafeJoinMethod(name string) (types.JoinMethod, error) {
	// When given a join method name that is already URL safe, just return it.
	if slices.Contains(onboarding.SupportedJoinMethods, name) {
		return types.JoinMethod(name), nil
	}

	// Various join methods contain underscores ("_") which are not valid
	// characters in URL schemes, and must be mapped from something valid.
	switch name {
	case "bound-keypair", "boundkeypair":
		return types.JoinMethodBoundKeypair, nil
	case "azure-devops", "azuredevops":
		return types.JoinMethodAzureDevops, nil
	case "terraform-cloud", "terraformcloud":
		return types.JoinMethodTerraformCloud, nil
	default:
		return types.JoinMethodUnspecified, trace.BadParameter("unsupported join method %q", name)
	}
}

// MapJoinMethodToURLSafe converts a join method name to a URL-safe string.
func MapJoinMethodToURLSafe(m types.JoinMethod) string {
	switch m {
	case types.JoinMethodBoundKeypair:
		return BoundKeypairSafeName
	case types.JoinMethodAzureDevops:
		return AzureDevopsSafeName
	case types.JoinMethodTerraformCloud:
		return TerraformCloudSafeName
	default:
		return string(m)
	}
}

// ParseJoinURI parses a joining URI from its string form. It returns an error
// if the input URI is malformed, missing parameters, or references an unknown
// or invalid join method or connection type.
func Parse(s string) (*JoinURI, error) {
	uri, err := url.Parse(s)
	if err != nil {
		return nil, trace.Wrap(err, "parsing joining URI")
	}

	schemeParts := strings.SplitN(uri.Scheme, "+", 3)
	if len(schemeParts) != 3 {
		return nil, trace.BadParameter("unsupported joining URI scheme: %q", uri.Scheme)
	}

	if schemeParts[0] != URISchemePrefix {
		return nil, trace.BadParameter(
			"unsupported joining URI scheme %q: scheme prefix must be %q",
			uri.Scheme, URISchemePrefix)
	}

	var kind connection.AddressKind
	switch schemeParts[1] {
	case string(connection.AddressKindProxy):
		kind = connection.AddressKindProxy
	case string(connection.AddressKindAuth):
		kind = connection.AddressKindAuth
	default:
		return nil, trace.BadParameter(
			"unsupported joining URI scheme %q: address kind must be one of [%q, %q], got: %q",
			uri.Scheme, connection.AddressKindProxy, connection.AddressKindAuth, schemeParts[1])
	}

	joinMethod, err := MapURLSafeJoinMethod(schemeParts[2])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if uri.User == nil {
		return nil, trace.BadParameter("invalid joining URI: must contain join token in user field")
	}

	param, _ := uri.User.Password()
	return &JoinURI{
		AddressKind:         kind,
		JoinMethod:          joinMethod,
		Token:               uri.User.Username(),
		JoinMethodParameter: param,
		Address:             uri.Host,
	}, nil
}

// FromProvisionToken returns a JoinURI for the given proxy address using fields
// from the given provision token where available.
func FromProvisionToken(token types.ProvisionToken, proxyAddr string) (*JoinURI, error) {
	ptv2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", token)
	}

	// Attempt to determine the join method parameter where possible. This is
	// method specific and occasionally refers to a client-side parameter, so it
	// cannot always be filled from information in the provision token.
	parameter := ""
	switch ptv2.GetJoinMethod() {
	case types.JoinMethodBoundKeypair:
		// Will be empty if already registered, or if a keypair was provided.
		parameter = ptv2.Status.BoundKeypair.RegistrationSecret
	}

	return &JoinURI{
		JoinMethod:          token.GetJoinMethod(),
		Token:               token.GetName(),
		JoinMethodParameter: parameter,
		AddressKind:         connection.AddressKindProxy,
		Address:             proxyAddr,
	}, nil
}
