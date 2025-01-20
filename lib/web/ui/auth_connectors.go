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

package ui

import (
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

// ListAuthConnectorsResponse describes a response to an auth connectors listing request
type ListAuthConnectorsResponse struct {
	// DefaultConnectorName is the name of the default auth connector in this cluster's auth preference.
	DefaultConnectorName string `json:"defaultConnectorName,omitempty"`
	// DefaultConnectorName is the name of the default auth connector in this cluster's auth preference.
	DefaultConnectorType string `json:"defaultConnectorType,omitempty"`
	// Connectors is the list of auth connectors.
	Connectors []ResourceItem `json:"connectors"`
}

// SetDefaultAuthConnectorRequest describes a request to set a default auth connector.
type SetDefaultAuthConnectorRequest struct {
	// Name is the name of the auth connector to set as default.
	Name string `json:"name"`
	// Type is the type of the auth connector to set as default.
	Type string `json:"type"`
}

// ValidConnectorTypes defines the allowed auth connector types
var ValidConnectorTypes = []string{
	constants.SAML,
	constants.OIDC,
	constants.Github,
	constants.LocalConnector,
}

// CheckAndSetDefaults checks if the provided values are valid.
func (r *SetDefaultAuthConnectorRequest) CheckAndSetDefaults() error {
	if r.Name == "" && r.Type != "local" {
		return trace.BadParameter("missing connector name")
	}
	if r.Type == "" {
		return trace.BadParameter("missing connector type")
	}

	if !slices.Contains(ValidConnectorTypes, r.Type) {
		return trace.BadParameter("unsupported connector type: %q", r.Type)
	}

	return nil
}
