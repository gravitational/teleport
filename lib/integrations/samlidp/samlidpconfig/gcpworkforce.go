/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package samlidpconfig

import (
	"regexp"
	"strconv"

	"github.com/gravitational/trace"
)

// GCPWorkforcePrams defines input params required
// to configure GCP Workforce Identity Federation pool
// and pool provider.
type GCPWorkforceAPIParams struct {
	// PoolName is a GCP workforce pool name.
	PoolName string
	// PoolProviderName is a GCP workforce pool provider name.
	PoolProviderName string
	// OrganizationID is a GCP organization ID.
	OrganizationID string
	// SAMLIdPMetadataURL is a URL path where Teleport proxy serves
	// the SAML IdP metadata.
	SAMLIdPMetadataURL string
}

func (p GCPWorkforceAPIParams) CheckAndSetDefaults() error {
	if err := validateGCPResourceName(p.PoolName, "PoolName"); err != nil {
		return trace.Wrap(err)
	}
	if err := validateGCPResourceName(p.PoolProviderName, "PoolProviderName"); err != nil {
		return trace.Wrap(err)
	}
	if err := validateOrganizationID(p.OrganizationID); err != nil {
		return trace.Wrap(err)
	}
	if p.SAMLIdPMetadataURL == "" {
		return trace.BadParameter("param SAMLIdPMetadataURL required")
	}

	return nil
}

// validateOrganizationID GCP organization ID which is all
// numeric value.
func validateOrganizationID(orgID string) error {
	if orgID == "" {
		return trace.BadParameter("param OrganizationID required.")
	}
	if _, err := strconv.Atoi(orgID); err != nil {
		return trace.BadParameter("organization ID must be of numeric value.")
	}

	return nil
}

var isValidGCPResourceName = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

// validateGCPResourceName validates name based on GCP naming convention.
// https://cloud.google.com/compute/docs/naming-resources#resource-name-format.
// paramFriendlyName is only in error message.
func validateGCPResourceName(param, paramFriendlyName string) error {
	if param == "" {
		return trace.BadParameter("param %s required", paramFriendlyName)
	}
	if len(param) > 63 {
		return trace.BadParameter("resource name cannot exceed 63 character length.")
	}

	if ok := isValidGCPResourceName.MatchString(param); !ok {
		return trace.BadParameter("resource name does not follow GCP resource naming convention.")
	}

	return nil
}
