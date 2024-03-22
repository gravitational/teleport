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
	out := &bytes.Buffer{}
	if err := gcpWIFConfigTemplate.Execute(out, p); err != nil {
		return "", trace.Wrap(err)
	}

	return out.String(), nil
}
