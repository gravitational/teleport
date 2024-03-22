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

package samlidp

import (
	"bytes"
	_ "embed"
	"regexp"
	"strconv"
	"text/template"

	"github.com/gravitational/trace"
)

var (
	//go:embed gcpwif.sh
	gcpWIFScript string

	// gcpWIFConfigTemplate
	gcpWIFConfigTemplate = template.Must(template.New("gcpwif").Parse(gcpWIFScript))
)

// GCPWIFConfigParams defines input params required to create bash script based
// gcpWIFConfigTemplate.
type GCPWIFConfigParams struct {
	OrgID            string
	PoolName         string
	PoolProviderName string
	MetadataEndpoint string
}

// BuildScript creates a Bash script that downloads Teleport
// SAML IdP metadata and runs gcloud command to create workforce
// identity pool and pool provider
func BuildScript(p GCPWIFConfigParams) (string, error) {
	if err := p.checkAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}

	out := &bytes.Buffer{}
	if err := gcpWIFConfigTemplate.Execute(out, p); err != nil {
		return "", trace.Wrap(err)
	}

	return out.String(), nil
}

// CheckAndSetDefaults checks if the required params are valid.
// GCP naming convention docs:
// https://cloud.google.com/compute/docs/naming-resources#resource-name-format
func (p *GCPWIFConfigParams) checkAndSetDefaults() error {
	if err := validateOrgID(p.OrgID); err != nil {
		return trace.Wrap(err)
	}

	if err := validateGCPResourceName(p.PoolName); err != nil {
		return trace.BadParameter("invalid pool name: %v", err)
	}

	if err := validateGCPResourceName(p.PoolProviderName); err != nil {
		return trace.BadParameter("invalid pool provider name: %v", err)
	}

	return nil
}

func validateOrgID(orgID string) error {
	if orgID == "" {
		return trace.BadParameter("organization ID is required.")
	}
	if _, err := strconv.Atoi(orgID); err != nil {
		return trace.BadParameter("organization ID must be of numeric value.")
	}

	return nil
}

var isValidGCPResourceName = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

func validateGCPResourceName(name string) error {
	if name == "" {
		return trace.BadParameter("name is empty.")
	}
	if len(name) > 63 {
		return trace.BadParameter("resource name cannot exceed 63 character length.")
	}

	if ok := isValidGCPResourceName.MatchString(name); !ok {
		return trace.BadParameter("resource name does not follow GCP resource naming convention.")
	}

	return nil
}
