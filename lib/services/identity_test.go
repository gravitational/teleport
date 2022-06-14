/*
Copyright 2022 Gravitational, Inc.

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

package services

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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
				ConnectorID: "foo",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: false,
		},
		{
			name: "below min CertTTL",
			req: types.SAMLAuthRequest{
				ConnectorID: "foo",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(1 * time.Second),
			},
			wantErr: true,
		},
		{
			name: "above max CertTTL",
			req: types.SAMLAuthRequest{
				ConnectorID: "foo",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(1000 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "TTL ignored without cert",
			req: types.SAMLAuthRequest{
				ConnectorID: "foo",
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: false,
		},
		{
			name: "SSOTestFlow requires ConnectorSpec",
			req: types.SAMLAuthRequest{
				ConnectorID: "foo",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
				SSOTestFlow: true,
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec requires SSOTestFlow",
			req: types.SAMLAuthRequest{
				ConnectorID:   "foo",
				PublicKey:     []byte(exampleSSHCert),
				CertTTL:       types.Duration(60 * time.Minute),
				ConnectorSpec: &types.SAMLConnectorSpecV2{Display: "dummy"},
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec with SSOTestFlow works",
			req: types.SAMLAuthRequest{
				ConnectorID:   "foo",
				PublicKey:     []byte(exampleSSHCert),
				CertTTL:       types.Duration(60 * time.Minute),
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
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: false,
		},
		{
			name: "missing state token",
			req: types.OIDCAuthRequest{
				ConnectorID: "foo",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: true,
		},
		{
			name: "below min CertTTL",
			req: types.OIDCAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(1 * time.Second),
			},
			wantErr: true,
		},
		{
			name: "above max CertTTL",
			req: types.OIDCAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(1000 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "TTL ignored without cert",
			req: types.OIDCAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: false,
		},
		{
			name: "SSOTestFlow requires ConnectorSpec",
			req: types.OIDCAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
				SSOTestFlow: true,
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec requires SSOTestFlow",
			req: types.OIDCAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				PublicKey:     []byte(exampleSSHCert),
				CertTTL:       types.Duration(60 * time.Minute),
				ConnectorSpec: &types.OIDCConnectorSpecV3{Display: "dummy"},
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec with SSOTestFlow works",
			req: types.OIDCAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				PublicKey:     []byte(exampleSSHCert),
				CertTTL:       types.Duration(60 * time.Minute),
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
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: false,
		},
		{
			name: "missing state token",
			req: types.GithubAuthRequest{
				ConnectorID: "foo",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: true,
		},
		{
			name: "below min CertTTL",
			req: types.GithubAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(1 * time.Second),
			},
			wantErr: true,
		},
		{
			name: "above max CertTTL",
			req: types.GithubAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(1000 * time.Hour),
			},
			wantErr: true,
		},
		{
			name: "TTL ignored without cert",
			req: types.GithubAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				CertTTL:     types.Duration(60 * time.Minute),
			},
			wantErr: false,
		},
		{
			name: "SSOTestFlow requires ConnectorSpec",
			req: types.GithubAuthRequest{
				ConnectorID: "foo",
				StateToken:  "bar",
				PublicKey:   []byte(exampleSSHCert),
				CertTTL:     types.Duration(60 * time.Minute),
				SSOTestFlow: true,
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec requires SSOTestFlow",
			req: types.GithubAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				PublicKey:     []byte(exampleSSHCert),
				CertTTL:       types.Duration(60 * time.Minute),
				ConnectorSpec: &types.GithubConnectorSpecV3{Display: "dummy"},
			},
			wantErr: true,
		},
		{
			name: "ConnectorSpec with SSOTestFlow works",
			req: types.GithubAuthRequest{
				ConnectorID:   "foo",
				StateToken:    "bar",
				PublicKey:     []byte(exampleSSHCert),
				CertTTL:       types.Duration(60 * time.Minute),
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
