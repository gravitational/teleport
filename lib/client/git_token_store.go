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

package client

import (
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalsecrets/v1"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/keypaths"
)

// SaveSessionCredentials saves a local cache of the session credentials.
// Only the credentials list is kept (with refresh tokens stripped).
// The encryption key and metadata are omitted.
func SaveSessionCredentials(homePath, proxyHost, username string, resource *pb.UserSessionCredentials) error {
	cacheResource := cloneForCache(resource)
	data, err := protojson.Marshal(cacheResource)
	if err != nil {
		return trace.Wrap(err)
	}
	path := keypaths.UserCredentialPath(profile.FullProfilePath(homePath), proxyHost, username)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.WriteFile(path, data, 0600))
}

// LoadSessionCredentials loads the cached session credentials.
func LoadSessionCredentials(homePath, proxyHost, username string) (*pb.UserSessionCredentials, error) {
	path := keypaths.UserCredentialPath(profile.FullProfilePath(homePath), proxyHost, username)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}
	var resource pb.UserSessionCredentials
	if err := protojson.Unmarshal(data, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resource, nil
}

// DeleteSessionCredentials removes the local credential cache file.
func DeleteSessionCredentials(homePath, proxyHost, username string) {
	path := keypaths.UserCredentialPath(profile.FullProfilePath(homePath), proxyHost, username)
	os.Remove(path)
}

// cloneForCache creates a minimal copy for local caching: keeps only the
// credentials list with refresh token blobs stripped.
func cloneForCache(resource *pb.UserSessionCredentials) *pb.UserSessionCredentials {
	var creds []*pb.Credential
	for _, c := range resource.GetSpec().GetCredentials() {
		cached := pb.Credential_builder{
			ResourceKind: c.GetResourceKind(),
			ResourceName: c.GetResourceName(),
		}
		if oauth := c.GetOauth(); oauth != nil {
			cached.Oauth = pb.OAuthSecret_builder{
				AccessTokenBlob:  oauth.GetAccessTokenBlob(),
				AccessTokenExpiry: oauth.GetAccessTokenExpiry(),
			}.Build()
		}
		creds = append(creds, cached.Build())
	}
	return pb.UserSessionCredentials_builder{
		Spec: pb.UserSessionCredentialsSpec_builder{
			Credentials: creds,
		}.Build(),
	}.Build()
}
