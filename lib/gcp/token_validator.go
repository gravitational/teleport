/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package gcp

import (
	"context"
	"fmt"
	"regexp"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/oidc"
)

const audience = "teleport.cluster.local"

// IDTokenValidatorConfig is the config for IDTokenValidator.
type IDTokenValidatorConfig struct {
	// issuerHost is the host of the Issuer for tokens issued by Google, to be
	// overridden in tests. Defaults to "accounts.google.com".
	issuerHost string
	// insecure configures the validator to use HTTP rather than HTTPS, to be
	// overridden in tests.
	insecure bool
}

// IDTokenValidator validates ID tokens from GCP.
type IDTokenValidator struct {
	IDTokenValidatorConfig
}

func NewIDTokenValidator(cfg IDTokenValidatorConfig) *IDTokenValidator {
	if cfg.issuerHost == "" {
		cfg.issuerHost = defaultIssuerHost
	}
	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

func (id *IDTokenValidator) issuerURL() string {
	scheme := "https"
	if id.insecure {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, id.issuerHost)
}

// Validate validates an ID token.
func (id *IDTokenValidator) Validate(ctx context.Context, token string) (*IDTokenClaims, error) {
	issuer := id.issuerURL()

	// GCP does not set the authorized party to one of the listed audiences, so we must skip the optional azp check.
	// TODO(Joerger): Use [rp.ValidateToken] once the authorized party check is made optional upstream, e.g. with an opt.
	claims, err := oidc.ValidateTokenNoAuthorizedPartyCheck[*IDTokenClaims](ctx, issuer, audience, token)
	if err != nil {
		return nil, trace.Wrap(err, "validating token")
	}

	if claims.Google.ComputeEngine != (ComputeEngine{}) {
		return claims, nil
	}

	if gcpDefaultServiceAccountEmailRegex.MatchString(claims.Email) {
		return nil, trace.BadParameter("default compute engine service account %q is not supported", claims.Email)
	}

	// GKE Workload Identity tokens do not have the `google.compute_engine` claim
	// and so to support Teleport services running on GKE, we need to extract the
	// project ID from the email claim.
	// Managed Service accounts running on GKE are not guaranteed to have domain emails
	// with the following format: <sa_name>@<project_id>.iam.gserviceaccount.com
	// See https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity#authenticating_to_other_services
	// See https://cloud.google.com/iam/docs/service-accounts#service_account_email_addresses
	// See https://cloud.google.com/iam/docs/service-account-types#user-managed
	matches := gcpUserManagedServiceAccountEmailRegex.FindStringSubmatch(claims.Email)
	if len(matches) != 2 {
		return nil, trace.BadParameter("invalid email claim: %q", claims.Email)
	}

	// Assign the project ID exatracted from the email to the Google claims.
	claims.Google.ComputeEngine.ProjectID = matches[1]

	return claims, nil
}

var (
	// gcpUserManagedServiceAccountEmailRegex is the regex used to extract the project ID from
	// the email claim of a GCP service account.
	// See https://cloud.google.com/iam/docs/service-accounts#service_account_email_addresses
	// See https://cloud.google.com/iam/docs/service-account-types#user-managed
	gcpUserManagedServiceAccountEmailRegex = regexp.MustCompile(`(?s)^[^@]+@([^\.]+)\.iam\.gserviceaccount\.com$`)

	// gcpDefaultServiceAccountEmailRegex is the regex used to identify when the default
	// compute engine service account is being used. When this is the case, we return an
	// error because the default compute engine service account is not supported.
	gcpDefaultServiceAccountEmailRegex = regexp.MustCompile(`(?s)^[^@]+@developer\.gserviceaccount\.com$`)
)
