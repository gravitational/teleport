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

package uri

import "github.com/gravitational/trace"

// ValidateFunc validates the provided ResourceURI.
type ValidateFunc func(ResourceURI) error

// Parse parses provided path as a cluster URI or a cluster resource URI.
func Parse(path string, validateFuncs ...ValidateFunc) (ResourceURI, error) {
	r := New(path)

	for _, validate := range append(
		[]ValidateFunc{validateProfileName}, // Basic validation.
		validateFuncs...,                    // Extra validations.
	) {
		if err := validate(r); err != nil {
			return ResourceURI{}, trace.Wrap(err)
		}
	}
	return r, nil
}

// ParseGatewayTargetURI parses the provided path as a gateway target URI.
func ParseGatewayTargetURI(path string) (ResourceURI, error) {
	r, err := Parse(path, validateGatewayTargetResource)
	return r, trace.Wrap(err)
}

// ParseDBURI parses the provided path as a database URI.
func ParseDBURI(path string) (ResourceURI, error) {
	r, err := Parse(path, validateDBResource)
	return r, trace.Wrap(err)
}

func validateProfileName(r ResourceURI) error {
	if r.GetProfileName() == "" {
		return trace.BadParameter("malformed URI %q, missing profile name", r)
	}
	return nil
}

func validateGatewayTargetResource(r ResourceURI) error {
	if r.GetDbName() == "" && r.GetKubeName() == "" && r.GetAppName() == "" {
		return trace.BadParameter("malformed gateway target URI %q, expecting a database, kube or app resource", r)
	}
	return nil
}

func validateDBResource(r ResourceURI) error {
	if r.GetDbName() == "" {
		return trace.BadParameter("malformed database URI %q, missing database name", r)
	}
	return nil
}
