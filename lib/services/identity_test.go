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

package services

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestSAMLAuthRequest_Check(t *testing.T) {
	const exampleSSHCert = `ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgb1srW/W3ZDjYAO45xLYAwzHBDLsJ4Ux6ICFIkTjb1LEAAAADAQABAAAAYQCkoR51poH0wE8w72cqSB8Sszx+vAhzcMdCO0wqHTj7UNENHWEXGrU0E0UQekD7U+yhkhtoyjbPOVIP7hNa6aRk/ezdh/iUnCIt4Jt1v3Z1h1P+hA4QuYFMHNB+rmjPwAcAAAAAAAAAAAAAAAEAAAAEdGVzdAAAAAAAAAAAAAAAAP//////////AAAAAAAAAIIAAAAVcGVybWl0LVgxMS1mb3J3YXJkaW5nAAAAAAAAABdwZXJtaXQtYWdlbnQtZm9yd2FyZGluZwAAAAAAAAAWcGVybWl0LXBvcnQtZm9yd2FyZGluZwAAAAAAAAAKcGVybWl0LXB0eQAAAAAAAAAOcGVybWl0LXVzZXItcmMAAAAAAAAAAAAAAHcAAAAHc3NoLXJzYQAAAAMBAAEAAABhANFS2kaktpSGc+CcmEKPyw9mJC4nZKxHKTgLVZeaGbFZOvJTNzBspQHdy7Q1uKSfktxpgjZnksiu/tFF9ngyY2KFoc+U88ya95IZUycBGCUbBQ8+bhDtw/icdDGQD5WnUwAAAG8AAAAHc3NoLXJzYQAAAGC8Y9Z2LQKhIhxf52773XaWrXdxP0t3GBVo4A10vUWiYoAGepr6rQIoGGXFxT4B9Gp+nEBJjOwKDXPrAevow0T9ca8gZN+0ykbhSrXLE5Ao48rqr3zP4O1/9P7e6gp0gw8=`

	tests := []struct {
		name    string
		req     types.SAMLAuthRequest
		wantErr bool
	}{
		{
			name: "normal request",
			req: types.SAMLAuthRequest{
				ConnectorID:  "foo",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "below min CertTTL",
			req: types.SAMLAuthRequest{
				ConnectorID:  "foo",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "above max CertTTL",
			req: types.SAMLAuthRequest{
				ConnectorID:  "foo",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      1000 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "TTL ignored without cert",
			req: types.SAMLAuthRequest{
				ConnectorID: "foo",
				CertTTL:     60 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "SSOTestFlow requires ConnectorSpec",
			req: types.SAMLAuthRequest{
				ConnectorID:  "foo",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
				SSOTestFlow:  true,
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec requires SSOTestFlow",
			req: types.SAMLAuthRequest{
				ConnectorID:   "foo",
				SshPublicKey:  []byte(exampleSSHCert),
				CertTTL:       60 * time.Minute,
				ConnectorSpec: &types.SAMLConnectorSpecV2{Display: "dummy"},
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec with SSOTestFlow works",
			req: types.SAMLAuthRequest{
				ConnectorID:   "foo",
				SshPublicKey:  []byte(exampleSSHCert),
				CertTTL:       60 * time.Minute,
				SSOTestFlow:   true,
				ConnectorSpec: &types.SAMLConnectorSpecV2{Display: "dummy"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Check()
			if tt.wantErr {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOIDCAuthRequest_Check(t *testing.T) {
	const exampleSSHCert = `ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgb1srW/W3ZDjYAO45xLYAwzHBDLsJ4Ux6ICFIkTjb1LEAAAADAQABAAAAYQCkoR51poH0wE8w72cqSB8Sszx+vAhzcMdCO0wqHTj7UNENHWEXGrU0E0UQekD7U+yhkhtoyjbPOVIP7hNa6aRk/ezdh/iUnCIt4Jt1v3Z1h1P+hA4QuYFMHNB+rmjPwAcAAAAAAAAAAAAAAAEAAAAEdGVzdAAAAAAAAAAAAAAAAP//////////AAAAAAAAAIIAAAAVcGVybWl0LVgxMS1mb3J3YXJkaW5nAAAAAAAAABdwZXJtaXQtYWdlbnQtZm9yd2FyZGluZwAAAAAAAAAWcGVybWl0LXBvcnQtZm9yd2FyZGluZwAAAAAAAAAKcGVybWl0LXB0eQAAAAAAAAAOcGVybWl0LXVzZXItcmMAAAAAAAAAAAAAAHcAAAAHc3NoLXJzYQAAAAMBAAEAAABhANFS2kaktpSGc+CcmEKPyw9mJC4nZKxHKTgLVZeaGbFZOvJTNzBspQHdy7Q1uKSfktxpgjZnksiu/tFF9ngyY2KFoc+U88ya95IZUycBGCUbBQ8+bhDtw/icdDGQD5WnUwAAAG8AAAAHc3NoLXJzYQAAAGC8Y9Z2LQKhIhxf52773XaWrXdxP0t3GBVo4A10vUWiYoAGepr6rQIoGGXFxT4B9Gp+nEBJjOwKDXPrAevow0T9ca8gZN+0ykbhSrXLE5Ao48rqr3zP4O1/9P7e6gp0gw8=`

	tests := []struct {
		name    string
		req     types.OIDCAuthRequest
		wantErr bool
	}{
		{
			name: "normal request",
			req: types.OIDCAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "missing state token",
			req: types.OIDCAuthRequest{
				ConnectorID:  "foo",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "below min CertTTL",
			req: types.OIDCAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "above max CertTTL",
			req: types.OIDCAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      1000 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "TTL ignored without cert",
			req: types.OIDCAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				CertTTL:     60 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "SSOTestFlow requires ConnectorSpec",
			req: types.OIDCAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
				SSOTestFlow:  true,
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec requires SSOTestFlow",
			req: types.OIDCAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				SshPublicKey:  []byte(exampleSSHCert),
				CertTTL:       60 * time.Minute,
				ConnectorSpec: &types.OIDCConnectorSpecV3{Display: "dummy"},
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec with SSOTestFlow works",
			req: types.OIDCAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				SshPublicKey:  []byte(exampleSSHCert),
				CertTTL:       60 * time.Minute,
				SSOTestFlow:   true,
				ConnectorSpec: &types.OIDCConnectorSpecV3{Display: "dummy"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Check()
			if tt.wantErr {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGithubAuthRequest_Check(t *testing.T) {
	const exampleSSHCert = `ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgb1srW/W3ZDjYAO45xLYAwzHBDLsJ4Ux6ICFIkTjb1LEAAAADAQABAAAAYQCkoR51poH0wE8w72cqSB8Sszx+vAhzcMdCO0wqHTj7UNENHWEXGrU0E0UQekD7U+yhkhtoyjbPOVIP7hNa6aRk/ezdh/iUnCIt4Jt1v3Z1h1P+hA4QuYFMHNB+rmjPwAcAAAAAAAAAAAAAAAEAAAAEdGVzdAAAAAAAAAAAAAAAAP//////////AAAAAAAAAIIAAAAVcGVybWl0LVgxMS1mb3J3YXJkaW5nAAAAAAAAABdwZXJtaXQtYWdlbnQtZm9yd2FyZGluZwAAAAAAAAAWcGVybWl0LXBvcnQtZm9yd2FyZGluZwAAAAAAAAAKcGVybWl0LXB0eQAAAAAAAAAOcGVybWl0LXVzZXItcmMAAAAAAAAAAAAAAHcAAAAHc3NoLXJzYQAAAAMBAAEAAABhANFS2kaktpSGc+CcmEKPyw9mJC4nZKxHKTgLVZeaGbFZOvJTNzBspQHdy7Q1uKSfktxpgjZnksiu/tFF9ngyY2KFoc+U88ya95IZUycBGCUbBQ8+bhDtw/icdDGQD5WnUwAAAG8AAAAHc3NoLXJzYQAAAGC8Y9Z2LQKhIhxf52773XaWrXdxP0t3GBVo4A10vUWiYoAGepr6rQIoGGXFxT4B9Gp+nEBJjOwKDXPrAevow0T9ca8gZN+0ykbhSrXLE5Ao48rqr3zP4O1/9P7e6gp0gw8=`

	tests := []struct {
		name    string
		req     types.GithubAuthRequest
		wantErr bool
	}{
		{
			name: "normal request",
			req: types.GithubAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "missing state token",
			req: types.GithubAuthRequest{
				ConnectorID:  "foo",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "below min CertTTL",
			req: types.GithubAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "above max CertTTL",
			req: types.GithubAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      1000 * time.Hour,
			},
			wantErr: true,
		},
		{
			name: "TTL ignored without cert",
			req: types.GithubAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				CertTTL:     60 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "SSOTestFlow requires ConnectorSpec",
			req: types.GithubAuthRequest{
				ConnectorID:  "foo",
				StateToken:   "bar",
				SshPublicKey: []byte(exampleSSHCert),
				CertTTL:      60 * time.Minute,
				SSOTestFlow:  true,
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec requires SSOTestFlow",
			req: types.GithubAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				SshPublicKey:  []byte(exampleSSHCert),
				CertTTL:       60 * time.Minute,
				ConnectorSpec: &types.GithubConnectorSpecV3{Display: "dummy"},
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec with SSOTestFlow works",
			req: types.GithubAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				SshPublicKey:  []byte(exampleSSHCert),
				CertTTL:       60 * time.Minute,
				SSOTestFlow:   true,
				ConnectorSpec: &types.GithubConnectorSpecV3{Display: "dummy"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Check()
			if tt.wantErr {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	tests := []struct {
		name    string
		pass    []byte
		wantErr bool
	}{
		{
			name:    "password too short",
			pass:    make([]byte, defaults.MinPasswordLength-1),
			wantErr: true,
		},
		{
			name:    "password just right",
			pass:    make([]byte, defaults.MinPasswordLength),
			wantErr: false,
		},
		{
			name:    "password too long",
			pass:    make([]byte, defaults.MaxPasswordLength+1),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyPassword(tt.pass)
			if tt.wantErr {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
