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

package services

import (
	userexternalcredentialsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
)

// FindUserExternalCredential returns the credential for the given resource.
func FindUserExternalCredential(resource *userexternalcredentialsv1.UserExternalCredentials, resourceKind, resourceName string) (*userexternalcredentialsv1.Credential, bool) {
	for _, credential := range resource.GetSpec().GetCredentials() {
		if credential.GetResourceKind() == resourceKind && credential.GetResourceName() == resourceName {
			return credential, true
		}
	}
	return nil, false
}
