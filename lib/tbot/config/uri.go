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

package config

import (
	"context"
	"log/slog"
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
)

type JoinURIParams struct {
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

// applyValueOrError sets the target `target` to the value `value`, but only if
// the current value is that type's zero value, or if the current value is equal
// to the desired value. If not, an error is returned per the error message
// string and arguments. This can be used to ensure existing values will not be
// overwritten.
func applyValueOrError[T comparable](target *T, value T, errMsg string, errArgs ...any) error {
	var zero T
	switch *target {
	case zero:
		*target = value
		return nil
	case value:
		return nil
	}

	return trace.BadParameter(errMsg, errArgs...)
}

// ApplyToConfig applies parameters from a parsed joining URI to the given bot
// config. This is designed to be applied to a configuration that has already
// been loaded - but not yet validated - and returns an error if any fields in
// the URI will conflict with those already set in the existing configuration.
func (p *JoinURIParams) ApplyToConfig(cfg *BotConfig) error {
	var errors []error

	if cfg.AuthServer != "" {
		errors = append(errors, trace.BadParameter("URI conflicts with configured field: auth_server"))
	} else if cfg.ProxyServer != "" {
		errors = append(errors, trace.BadParameter("URI conflicts with configured field: proxy_server"))
	} else {
		switch p.AddressKind {
		case connection.AddressKindAuth:
			cfg.AuthServer = p.Address
		default:
			// this parameter should not be unspecified due to checks in
			// ParseJoinURI, so we'll assume proxy.
			cfg.ProxyServer = p.Address
		}
	}

	errors = append(errors, applyValueOrError(
		&cfg.Onboarding.JoinMethod, p.JoinMethod,
		"URI joining method %q conflicts with configured field: onboarding.join_method", p.JoinMethod))

	if cfg.Onboarding.TokenValue != "" {
		errors = append(errors, trace.BadParameter("URI conflicts with configured field: onboarding.token"))
	} else {
		cfg.Onboarding.SetToken(p.Token)
	}

	// The join method parameter maps to a method-specific field when set.
	if param := p.JoinMethodParameter; param != "" {
		switch p.JoinMethod {
		case types.JoinMethodAzure:
			errors = append(errors, applyValueOrError(
				&cfg.Onboarding.Azure.ClientID, param,
				"URI join method parameter %q conflicts with configured field: onboarding.azure.client_id",
				param))
		case types.JoinMethodTerraformCloud:
			errors = append(errors, applyValueOrError(
				&cfg.Onboarding.Terraform.AudienceTag, param,
				"URI join method parameter %q conflicts with configured field: onboarding.terraform.audience_tag", param))
		case types.JoinMethodGitLab:
			errors = append(errors, applyValueOrError(
				&cfg.Onboarding.Gitlab.TokenEnvVarName, param,
				"URI join method parameter %q conflicts with configured field: onboarding.gitlab.token_env_var_name", param))
		case types.JoinMethodBoundKeypair:
			errors = append(errors, applyValueOrError(
				&cfg.Onboarding.BoundKeypair.RegistrationSecretValue, param,
				"URI join method parameter %q conflicts with configured field: onboarding.bound_keypair.registration_secret", param))
		default:
			slog.WarnContext(
				context.Background(),
				"ignoring join method parameter for unsupported join method",
				"join_method", p.JoinMethod,
			)
		}
	}

	return trace.NewAggregate(errors...)
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

// ParseJoinURI parses a joining URI from its string form. It returns an error
// if the input URI is malformed, missing parameters, or references an unknown
// or invalid join method or connection type.
func ParseJoinURI(s string) (*JoinURIParams, error) {
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
	return &JoinURIParams{
		AddressKind:         kind,
		JoinMethod:          joinMethod,
		Token:               uri.User.Username(),
		JoinMethodParameter: param,
		Address:             uri.Host,
	}, nil
}
