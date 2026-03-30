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
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/join/provision"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "gcp")

// Validator is an interface for GCP token validators, used in tests to provide
// mock implementations.
type Validator interface {
	Validate(ctx context.Context, token string) (*IDTokenClaims, error)
}

// defaultIssuerHost is the issuer for GCP ID tokens.
const defaultIssuerHost = "accounts.google.com"

// ComputeEngine contains VM-specific token claims.
type ComputeEngine struct {
	// The ID of the instance's project.
	ProjectID string `json:"project_id"`
	// The instance's zone.
	Zone string `json:"zone"`
	// The instance's ID.
	InstanceID string `json:"instance_id"`
	// The instance's name.
	InstanceName string `json:"instance_name"`
}

// Google contains Google-specific token claims.
type Google struct {
	ComputeEngine ComputeEngine `json:"compute_engine"`
}

// IDTokenClaims is the set of claims in a GCP ID token. GCP documentation for
// claims can be found at
// https://cloud.google.com/compute/docs/instances/verifying-instance-identity#payload
type IDTokenClaims struct {
	oidc.TokenClaims
	// The email of the service account that this token was issued for.
	Email  string `json:"email"`
	Google Google `json:"google"`
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsGCP {
	attrs := &workloadidentityv1pb.JoinAttrsGCP{
		ServiceAccount: c.Email,
	}
	if c.Google.ComputeEngine.InstanceName != "" {
		attrs.Gce = &workloadidentityv1pb.JoinAttrsGCPGCE{
			Project: c.Google.ComputeEngine.ProjectID,
			Zone:    c.Google.ComputeEngine.Zone,
			Id:      c.Google.ComputeEngine.InstanceID,
			Name:    c.Google.ComputeEngine.InstanceName,
		}
	}

	return attrs
}

// CheckIDTokenParams are parameters used to validate GCP OIDC tokens.
type CheckIDTokenParams struct {
	ProvisionToken provision.Token
	IDToken        []byte
	Validator      Validator
}

func (p *CheckIDTokenParams) validate() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case len(p.IDToken) == 0:
		return trace.BadParameter("IDToken is required")
	case p.Validator == nil:
		return trace.BadParameter("Validator is required")
	}
	return nil
}

// CheckIDToken verifies a GCP OIDC token
func CheckIDToken(
	ctx context.Context,
	params *CheckIDTokenParams,
) (*IDTokenClaims, error) {
	if err := params.validate(); err != nil {
		return nil, trace.AccessDenied("%s", err.Error())
	}

	claims, err := params.Validator.Validate(ctx, string(params.IDToken))
	if err != nil {
		log.WarnContext(ctx, "Unable to validate GCP IDToken",
			"error", err,
			"claims", claims,
			"token", params.ProvisionToken.GetName(),
		)
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "GCP VM trying to join cluster",
		"claims", claims,
		"token", params.ProvisionToken.GetName(),
	)

	// Note: try to return claims even in case of error to improve logging on
	// failed auth attempts.
	return claims, trace.Wrap(checkGCPAllowRules(params.ProvisionToken, claims))
}

func checkGCPAllowRules(token provision.Token, claims *IDTokenClaims) error {
	compute := claims.Google.ComputeEngine
	// unmatchedLocation is true if the location restriction is set and the "google.compute_engine.zone"
	// claim is not present in the IDToken. This happens when the joining node is not a GCE VM.
	unmatchedLocation := false
	// If a single rule passes, accept the IDToken.
	for _, rule := range token.GetGCPRules().Allow {
		if !slices.Contains(rule.ProjectIDs, compute.ProjectID) {
			continue
		}

		if len(rule.ServiceAccounts) > 0 && !slices.Contains(rule.ServiceAccounts, claims.Email) {
			continue
		}

		if len(rule.Locations) > 0 && !slices.ContainsFunc(rule.Locations, func(location string) bool {
			return isGCPZoneInLocation(location, compute.Zone)
		}) {
			unmatchedLocation = true
			continue
		}

		// All provided rules met.
		return nil
	}

	// If the location restriction is set and the "google.compute_engine.zone" claim is not present in the IDToken,
	// return a more specific error message.
	if unmatchedLocation && compute.Zone == "" {
		return trace.CompareFailed("id token %q claim is empty and didn't match the %q. "+
			"Services running outside of GCE VM instances are incompatible with %q restriction.", "google.compute_engine.zone", "locations", "location")
	}
	return trace.AccessDenied("id token claims did not match any allow rules")
}

type gcpLocation struct {
	globalLocation string
	region         string
	zone           string
}

func parseGCPLocation(location string) (*gcpLocation, error) {
	parts := strings.Split(location, "-")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, trace.BadParameter("location %q is not a valid GCP region or zone", location)
	}
	globalLocation, region := parts[0], parts[1]
	var zone string
	if len(parts) == 3 {
		zone = parts[2]
	}
	return &gcpLocation{
		globalLocation: globalLocation,
		region:         region,
		zone:           zone,
	}, nil
}

// isGCPZoneInLocation checks if a zone belongs to a location, which can be
// either a zone or region.
func isGCPZoneInLocation(rawLocation, rawZone string) bool {
	location, err := parseGCPLocation(rawLocation)
	if err != nil {
		return false
	}
	zone, err := parseGCPLocation(rawZone)
	if err != nil {
		return false
	}
	// Make sure zone is, in fact, a zone.
	if zone.zone == "" {
		return false
	}

	if location.globalLocation != zone.globalLocation || location.region != zone.region {
		return false
	}
	return location.zone == "" || location.zone == zone.zone
}
