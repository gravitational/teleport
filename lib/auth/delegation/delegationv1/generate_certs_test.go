/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package delegationv1_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/client/proto"
	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/delegation/delegationv1"
	"github.com/gravitational/teleport/lib/auth/internal"
	"github.com/gravitational/trace"
)

var (
	sshCACertificates = [][]byte{[]byte("SSH CA CERTIFICATE")}
	sshCertificate    = []byte("SSH CERTIFICATE")
	sshPublicKey      = []byte("SSH PUBLIC KEY")

	tlsCACertificates = [][]byte{[]byte("TLS CA CERTIFICATE")}
	tlsCertificate    = []byte("TLS CERTIFICATE")
	tlsPublicKey      = []byte("TLS PUBLIC KEY")
)

func TestSessionService_GenerateCerts(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			service, pack := sessionServiceTestPack(t)
			pack.createUser(t, "bob", types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						"*": []string{"*"},
					},
				},
			})

			appSession, err := types.NewWebSession(
				uuid.NewString(),
				types.KindAppSession,
				types.WebSessionSpecV2{
					User: "bob",
				},
			)
			require.NoError(t, err)
			pack.onCreateAppSession = func(_ context.Context, req internal.NewAppSessionRequest) (types.WebSession, error) {
				require.Equal(t, "bob", req.User)
				require.Equal(t, []string{"bob"}, req.Roles)
				require.Equal(t,
					[]types.ResourceID{
						{
							ClusterName: "test.teleport.sh",
							Kind:        "app",
							Name:        "hr-system",
						},
					},
					req.RequestedResourceIDs,
				)
				return appSession, nil
			}

			pack.onGenerateCert = func(_ context.Context, req internal.CertRequest) (*proto.Certs, error) {
				require.Equal(t, "bob", req.User.GetName())
				require.Equal(t, sshPublicKey, req.SSHPublicKey)
				require.Equal(t, tlsPublicKey, req.TLSPublicKey)
				require.Equal(t, 5*time.Minute, req.TTL)

				// Check roles and resource IDs.
				checker, ok := req.Checker.Unscoped()
				require.True(t, ok)
				require.Equal(t, []string{"bob"}, checker.RoleNames())
				require.Equal(t,
					[]types.ResourceID{
						{
							ClusterName: "test.teleport.sh",
							Kind:        "app",
							Name:        "hr-system",
						},
					},
					checker.GetAllowedResourceIDs(),
				)

				// Check the app session.
				require.Equal(t, appSession.GetName(), req.AppSessionID)
				require.Equal(t, "payroll-agent", req.BotName)

				return &proto.Certs{
					TLS:        tlsCertificate,
					SSH:        sshCertificate,
					TLSCACerts: tlsCACertificates,
					SSHCACerts: sshCACertificates,
				}, nil
			}
			pack.authenticateBot("payroll-agent")

			session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
				User: "bob",
				Resources: []*delegationv1pb.DelegationResourceSpec{
					{
						Kind: types.KindApp,
						Name: "hr-system",
					},
				},
				AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
					{
						Type: types.DelegationUserTypeBot,
						Matcher: &delegationv1pb.DelegationUserSpec_BotName{
							BotName: "payroll-agent",
						},
					},
				},
			})

			rsp, err := service.GenerateCerts(t.Context(), &delegationv1pb.GenerateCertsRequest{
				DelegationSessionId: session.GetMetadata().GetName(),
				SshPublicKey:        sshPublicKey,
				TlsPublicKey:        tlsPublicKey,
				Expires:             timestamppb.New(time.Now().Add(5 * time.Minute)),
				Routing: &delegationv1pb.GenerateCertsRequest_RouteToApp{
					RouteToApp: &delegationv1pb.RouteToApp{
						Name:        "hr-system",
						PublicAddr:  "hr-system.test.teleport.sh",
						ClusterName: "test.teleport.sh",
						Uri:         "http://localhost:9000",
						TargetPort:  9000,
					},
				},
			})
			require.NoError(t, err)
			require.Equal(t,
				&delegationv1pb.GenerateCertsResponse{
					Ssh:    sshCertificate,
					Tls:    tlsCertificate,
					SshCas: sshCACertificates,
					TlsCas: tlsCACertificates,
				},
				rsp,
			)
		})
	})

	t.Run("session not found", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)
		pack.authenticateBot("payroll-agent")

		_, err := service.GenerateCerts(t.Context(), &delegationv1pb.GenerateCertsRequest{
			DelegationSessionId: uuid.NewString(),
			SshPublicKey:        sshPublicKey,
			TlsPublicKey:        tlsPublicKey,
			Expires:             timestamppb.New(time.Now().Add(1 * time.Hour)),
		})
		require.ErrorIs(t, err, delegationv1.ErrDelegationUnauthorized)
	})

	t.Run("wrong bot", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)
		pack.createUser(t, "bob", types.RoleSpecV6{
			Allow: types.RoleConditions{
				AppLabels: types.Labels{
					"*": []string{"*"},
				},
			},
		})
		pack.authenticateBot("evil-bot")

		session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
			User: "bob",
			Resources: []*delegationv1pb.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "hr-system",
				},
			},
			AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
				{
					Type: types.DelegationUserTypeBot,
					Matcher: &delegationv1pb.DelegationUserSpec_BotName{
						BotName: "payroll-agent",
					},
				},
			},
		})

		_, err := service.GenerateCerts(t.Context(), &delegationv1pb.GenerateCertsRequest{
			DelegationSessionId: session.GetMetadata().GetName(),
			SshPublicKey:        sshPublicKey,
			TlsPublicKey:        tlsPublicKey,
			Expires:             timestamppb.New(time.Now().Add(1 * time.Hour)),
		})
		require.ErrorIs(t, err, delegationv1.ErrDelegationUnauthorized)
	})

	t.Run("session expired", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			service, pack := sessionServiceTestPack(t)
			pack.createUser(t, "bob", types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						"*": []string{"*"},
					},
				},
			})
			pack.authenticateBot("payroll-agent")

			session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
				User: "bob",
				Resources: []*delegationv1pb.DelegationResourceSpec{
					{
						Kind: types.KindApp,
						Name: "hr-system",
					},
				},
				AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
					{
						Type: types.DelegationUserTypeBot,
						Matcher: &delegationv1pb.DelegationUserSpec_BotName{
							BotName: "payroll-agent",
						},
					},
				},
			})

			// This won't actually sleep, thanks to synctest!
			time.Sleep(time.Until(session.GetMetadata().GetExpires().AsTime()))

			_, err := service.GenerateCerts(t.Context(), &delegationv1pb.GenerateCertsRequest{
				DelegationSessionId: session.GetMetadata().GetName(),
				SshPublicKey:        sshPublicKey,
				TlsPublicKey:        tlsPublicKey,
				Expires:             timestamppb.New(time.Now().Add(1 * time.Hour)),
			})
			require.ErrorIs(t, err, delegationv1.ErrDelegationUnauthorized)
		})
	})

	t.Run("missing resource access", func(t *testing.T) {
		service, pack := sessionServiceTestPack(t)
		pack.createUser(t, "bob", types.RoleSpecV6{})
		pack.authenticateBot("payroll-agent")

		session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
			User: "bob",
			Resources: []*delegationv1pb.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "hr-system",
				},
			},
			AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
				{
					Type: types.DelegationUserTypeBot,
					Matcher: &delegationv1pb.DelegationUserSpec_BotName{
						BotName: "payroll-agent",
					},
				},
			},
		})

		_, err := service.GenerateCerts(t.Context(), &delegationv1pb.GenerateCertsRequest{
			DelegationSessionId: session.GetMetadata().GetName(),
			SshPublicKey:        sshPublicKey,
			TlsPublicKey:        tlsPublicKey,
			Expires:             timestamppb.New(time.Now().Add(1 * time.Hour)),
		})
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "You do not have permission to delegate access to all of the required resources")
	})

	t.Run("expires greater than session expiry", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			service, pack := sessionServiceTestPack(t)
			pack.createUser(t, "bob", types.RoleSpecV6{
				Allow: types.RoleConditions{
					AppLabels: types.Labels{
						"*": []string{"*"},
					},
				},
			})

			appSession, err := types.NewWebSession(
				uuid.NewString(),
				types.KindAppSession,
				types.WebSessionSpecV2{
					User: "bob",
				},
			)
			require.NoError(t, err)
			pack.onCreateAppSession = func(_ context.Context, req internal.NewAppSessionRequest) (types.WebSession, error) {
				require.Equal(t, 1*time.Hour, req.SessionTTL) // This is the session expiry, not the requested TTL.
				return appSession, nil
			}

			pack.onGenerateCert = func(_ context.Context, req internal.CertRequest) (*proto.Certs, error) {
				require.Equal(t, 1*time.Hour, req.TTL) // This is the session expiry, not the requested TTL.

				return &proto.Certs{
					TLS:        tlsCertificate,
					SSH:        sshCertificate,
					TLSCACerts: tlsCACertificates,
					SSHCACerts: sshCACertificates,
				}, nil
			}
			pack.authenticateBot("payroll-agent")

			session := pack.createSession(t, &delegationv1pb.DelegationSessionSpec{
				User: "bob",
				Resources: []*delegationv1pb.DelegationResourceSpec{
					{
						Kind: types.KindApp,
						Name: "hr-system",
					},
				},
				AuthorizedUsers: []*delegationv1pb.DelegationUserSpec{
					{
						Type: types.DelegationUserTypeBot,
						Matcher: &delegationv1pb.DelegationUserSpec_BotName{
							BotName: "payroll-agent",
						},
					},
				},
			})

			_, err = service.GenerateCerts(t.Context(), &delegationv1pb.GenerateCertsRequest{
				DelegationSessionId: session.GetMetadata().GetName(),
				SshPublicKey:        sshPublicKey,
				TlsPublicKey:        tlsPublicKey,
				Expires:             timestamppb.New(time.Now().Add(365 * 24 * 1 * time.Hour)),
				Routing: &delegationv1pb.GenerateCertsRequest_RouteToApp{
					RouteToApp: &delegationv1pb.RouteToApp{
						Name:        "hr-system",
						PublicAddr:  "hr-system.test.teleport.sh",
						ClusterName: "test.teleport.sh",
						Uri:         "http://localhost:9000",
						TargetPort:  9000,
					},
				},
			})
			require.NoError(t, err)
		})
	})
}
