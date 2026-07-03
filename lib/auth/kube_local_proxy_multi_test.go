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

package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	authpb "github.com/gravitational/teleport/api/client/proto"
)

func TestValidateUserCertsRequestKubeLocalProxyMulti(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc       string
		req        *authpb.UserCertsRequest
		wantErrMsg string
	}{
		{
			desc: "unrouted kube cert allowed for multi requester",
			req: &authpb.UserCertsRequest{
				Usage:         authpb.UserCertsRequest_Kubernetes,
				RequesterName: authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
			},
		},
		{
			desc: "routed kube cert also allowed for multi requester",
			req: &authpb.UserCertsRequest{
				Usage:             authpb.UserCertsRequest_Kubernetes,
				KubernetesCluster: "kube-a",
				RequesterName:     authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
			},
		},
		{
			desc: "non-kube usage rejected for multi requester",
			req: &authpb.UserCertsRequest{
				Usage:         authpb.UserCertsRequest_App,
				RouteToApp:    authpb.RouteToApp{Name: "app-a"},
				RequesterName: authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
			},
			wantErrMsg: "can only request Kubernetes certificates",
		},
		{
			desc: "MFA response rejected for multi requester",
			req: &authpb.UserCertsRequest{
				Usage:         authpb.UserCertsRequest_Kubernetes,
				RequesterName: authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
				MFAResponse:   &authpb.MFAAuthenticateResponse{},
			},
			wantErrMsg: "cannot request MFA-verified certificates",
		},
		{
			desc: "unrouted kube cert still rejected for other requesters",
			req: &authpb.UserCertsRequest{
				Usage:         authpb.UserCertsRequest_Kubernetes,
				RequesterName: authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY,
			},
			wantErrMsg: "missing KubernetesCluster field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// srv is only consulted for single-use cert requests, which the multi
			// requester never sends.
			err := validateUserCertsRequest(nil, tt.req)
			if tt.wantErrMsg == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErrMsg)
		})
	}
}

func TestIsInMemoryCertRequestKubeLocalProxyMulti(t *testing.T) {
	t.Parallel()

	require.True(t, isInMemoryCertRequest(&authpb.UserCertsRequest{
		Usage:         authpb.UserCertsRequest_Kubernetes,
		RequesterName: authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
	}), "shared multi-cluster certs must stay in memory and keep the session TTL")

	require.False(t, isInMemoryCertRequest(&authpb.UserCertsRequest{
		Usage:         authpb.UserCertsRequest_Database,
		RequesterName: authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
	}))
}
