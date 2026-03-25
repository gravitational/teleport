/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	joinuri "github.com/gravitational/teleport/lib/tbot/config/joinuri"
)

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
func ApplyJoinURIToConfig(uri *joinuri.JoinURI, cfg *BotConfig) error {
	var errors []error

	if cfg.AuthServer != "" {
		errors = append(errors, trace.BadParameter("URI conflicts with configured field: auth_server"))
	} else if cfg.ProxyServer != "" {
		errors = append(errors, trace.BadParameter("URI conflicts with configured field: proxy_server"))
	} else {
		switch uri.AddressKind {
		case connection.AddressKindAuth:
			cfg.AuthServer = uri.Address
		default:
			// this parameter should not be unspecified due to checks in
			// ParseJoinURI, so we'll assume proxy.
			cfg.ProxyServer = uri.Address
		}
	}

	errors = append(errors, applyValueOrError(
		&cfg.Onboarding.JoinMethod, uri.JoinMethod,
		"URI joining method %q conflicts with configured field: onboarding.join_method", uri.JoinMethod))

	if cfg.Onboarding.TokenValue != "" {
		errors = append(errors, trace.BadParameter("URI conflicts with configured field: onboarding.token"))
	} else {
		cfg.Onboarding.SetToken(uri.Token)
	}

	// The join method parameter maps to a method-specific field when set.
	if param := uri.JoinMethodParameter; param != "" {
		switch uri.JoinMethod {
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
				"join_method", uri.JoinMethod,
			)
		}
	}

	return trace.NewAggregate(errors...)
}
