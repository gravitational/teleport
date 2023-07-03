/*
Copyright 2023 Gravitational, Inc.

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

package auth

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/gcp"
)

type gcpIDTokenValidator interface {
	Validate(ctx context.Context, token string) (*gcp.IDTokenClaims, error)
}

func (a *Server) checkGCPJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) error {
	if req.IDToken == "" {
		return trace.BadParameter("IDToken not provided for GCP join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return trace.Wrap(err)
	}

	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return trace.BadParameter("gcp join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	claims, err := a.gcpIDTokenValidator.Validate(ctx, req.IDToken)
	if err != nil {
		return trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  pt.GetName(),
	}).Info("GCP VM trying to join cluster")

	return trace.Wrap(checkGCPAllowRules(token, claims))
}

func checkGCPAllowRules(token *types.ProvisionTokenV2, claims *gcp.IDTokenClaims) error {
	compute := claims.Google.ComputeEngine
	// If a single rule passes, accept the IDToken.
	for _, rule := range token.Spec.GCP.Allow {
		if !slices.Contains(rule.ProjectIDs, compute.ProjectID) {
			continue
		}
		if len(rule.Locations) > 0 && !slices.ContainsFunc(rule.Locations, func(location string) bool {
			return isGCPZoneInLocation(location, compute.Zone)
		}) {
			continue
		}
		if len(rule.ServiceAccounts) > 0 && !slices.Contains(rule.ServiceAccounts, claims.Email) {
			continue
		}

		// All provided rules met.
		return nil
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
