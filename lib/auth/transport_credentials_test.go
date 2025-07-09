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

package auth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// TestTransportCredentials_Check validates the returned values
// from [TransportCredentialsConfig.Check] based on various inputs.
func TestTransportCredentials_Check(t *testing.T) {
	tlsConf := newTLSConfig(t)

	cases := []struct {
		name                string
		tlsConf             *tls.Config
		conf                TransportCredentialsConfig
		errAssertion        require.ErrorAssertionFunc
		credentialAssertion require.ValueAssertionFunc
	}{
		{
			name: "invalid configuration: no transport credentials",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("parameter TransportCredentials required"))
			},
			credentialAssertion: require.Nil,
		},
		{
			name: "invalid configuration: bad transport credentials security protocol",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("the TransportCredentials must be a tls security protocol, got insecure"))
			},
			conf:                TransportCredentialsConfig{TransportCredentials: insecure.NewCredentials()},
			credentialAssertion: require.Nil,
		},
		{
			name: "invalid configuration: no user getter provided",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("parameter UserGetter required"))
			},
			conf:                TransportCredentialsConfig{TransportCredentials: credentials.NewTLS(tlsConf.Clone())},
			credentialAssertion: require.Nil,
		},
		{
			name: "invalid configuration: connection enforcer provided without an authorizer",
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("both a UserGetter and an Authorizer are required to enforce connection limits with an Enforcer"))
			},
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{},
				Enforcer:             &fakeEnforcer{},
			},
			credentialAssertion: require.Nil,
		},
		{
			name:         "valid configuration: without connection limiter or authorizer",
			errAssertion: require.NoError,
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{},
			},
			credentialAssertion: require.NotNil,
		},
		{
			name:         "valid configuration: without connection limiter",
			errAssertion: require.NoError,
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{},
				Authorizer:           &fakeAuthorizer{},
			},
			credentialAssertion: require.NotNil,
		},
		{
			name:         "valid configuration",
			errAssertion: require.NoError,
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{},
				Authorizer:           &fakeAuthorizer{},
			},
			credentialAssertion: require.NotNil,
		},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			creds, err := NewTransportCredentials(test.conf)
			test.errAssertion(t, err)
			test.credentialAssertion(t, creds)
		})
	}
}

// TestTransportCredentials_ServerHandshake validates that the [TransportCredentials.ServerHandshake]
// behaves as expected based on the TransportCredentialsConfig that were used to populate it and the
// [tls.Config] used by the client.
func TestTransportCredentials_ServerHandshake(t *testing.T) {
	t.Parallel()

	unauthorized := trace.AccessDenied("not authorized")
	tooManyConnections := trace.LimitExceeded("too many connections")
	tlsConf := newTLSConfig(t)

	cases := []struct {
		name               string
		conf               TransportCredentialsConfig
		clientTLSConf      *tls.Config
		errAssertion       require.ErrorAssertionFunc
		handshakeAssertion require.ErrorAssertionFunc
		infoAssertion      func(t *testing.T, info credentials.AuthInfo)
	}{
		{
			name: "valid connection without session control",
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{ClusterName: "test"},
			},
			clientTLSConf:      &tls.Config{InsecureSkipVerify: true},
			errAssertion:       require.NoError,
			handshakeAssertion: require.NoError,
			infoAssertion: func(t *testing.T, info credentials.AuthInfo) {
				identityInfo, ok := info.(IdentityInfo)
				require.True(t, ok)
				require.NotNil(t, identityInfo.TLSInfo)
				require.NotNil(t, identityInfo.IdentityGetter)
				require.NotNil(t, identityInfo.AuthContext)
				require.NotNil(t, identityInfo.AuthContext.Identity)
				require.Nil(t, identityInfo.AuthContext.Checker)
			},
		},
		{
			name: "valid connection with authorization but no connection limiting",
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{ClusterName: "test"},
				Authorizer:           &fakeAuthorizer{},
			},
			clientTLSConf:      &tls.Config{InsecureSkipVerify: true},
			errAssertion:       require.NoError,
			handshakeAssertion: require.NoError,
			infoAssertion: func(t *testing.T, info credentials.AuthInfo) {
				identityInfo, ok := info.(IdentityInfo)
				require.True(t, ok)
				require.NotNil(t, identityInfo.IdentityGetter)
				require.NotNil(t, identityInfo.AuthContext)
			},
		},
		{
			name: "valid connection with full session control",
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{ClusterName: "test"},
				Authorizer: &fakeAuthorizer{
					checker: &fakeChecker{maxConnections: 1},
				},
				Enforcer: &fakeEnforcer{},
			},
			clientTLSConf:      &tls.Config{InsecureSkipVerify: true},
			errAssertion:       require.NoError,
			handshakeAssertion: require.NoError,
			infoAssertion: func(t *testing.T, info credentials.AuthInfo) {
				identityInfo, ok := info.(IdentityInfo)
				require.True(t, ok)
				require.NotNil(t, identityInfo.IdentityGetter)
				require.NotNil(t, identityInfo.AuthContext)
			},
		},
		{
			name: "not authorized",
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{ClusterName: "test"},
				Authorizer:           &fakeAuthorizer{authorizeError: unauthorized},
			},
			clientTLSConf: &tls.Config{InsecureSkipVerify: true},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, unauthorized)
			},
			handshakeAssertion: require.NoError,
			infoAssertion: func(t *testing.T, info credentials.AuthInfo) {
				require.Nil(t, info)
			},
		},
		{
			name: "connection limits exceeded",
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{ClusterName: "test"},
				Authorizer:           &fakeAuthorizer{checker: &fakeChecker{maxConnections: 1}},
				Enforcer:             &fakeEnforcer{err: tooManyConnections},
			},
			clientTLSConf: &tls.Config{InsecureSkipVerify: true},
			errAssertion: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, tooManyConnections)
			},
			handshakeAssertion: require.NoError,
			infoAssertion: func(t *testing.T, info credentials.AuthInfo) {
				require.Nil(t, info)
			},
		},
		{
			name: "tls handshake failure",
			conf: TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(tlsConf.Clone()),
				UserGetter:           &Middleware{ClusterName: "test"},
			},
			clientTLSConf:      &tls.Config{InsecureSkipVerify: false},
			handshakeAssertion: require.Error,
		},
	}

	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, ln.Close())
			})

			creds, err := NewTransportCredentials(test.conf)
			require.NoError(t, err)

			errC := make(chan error, 1)
			doneC := make(chan credentials.AuthInfo, 1)
			go func() {
				conn, err := ln.Accept()
				if err != nil {
					errC <- err
					return
				}
				defer conn.Close()

				conn, info, err := creds.ServerHandshake(conn)
				if err != nil {
					errC <- err
					return
				}

				conn.Close()
				doneC <- info
			}()

			conn, err := net.Dial("tcp", ln.Addr().String())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, conn.Close()) })

			// this would be done by the grpc TransportCredential in the grpc
			// client, but we're going to fake it with just a tls.Client, so we
			// have to add the http2 next proto ourselves (enforced by grpc-go
			// starting from v1.67, and required by the http2 spec when speaking
			// http2 in TLS)
			clientTLSConf := test.clientTLSConf
			if !slices.Contains(clientTLSConf.NextProtos, "h2") {
				clientTLSConf = clientTLSConf.Clone()
				clientTLSConf.NextProtos = append(clientTLSConf.NextProtos, "h2")
			}
			clientConn := tls.Client(conn, clientTLSConf)

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			err = clientConn.HandshakeContext(ctx)
			test.handshakeAssertion(t, err)

			if err != nil {
				return
			}

			select {
			case err := <-errC:
				test.errAssertion(t, err)
			case info := <-doneC:
				test.infoAssertion(t, info)

			}
		})
	}
}

type fakeUserGetter struct {
	identity authz.IdentityGetter
}

func (f fakeUserGetter) GetUser(tls.ConnectionState) (authz.IdentityGetter, error) {
	return f.identity, nil
}

func TestTransportCredentialsDisconnection(t *testing.T) {
	cases := []struct {
		name   string
		expiry time.Duration
	}{
		{
			name: "no expiry",
		},
		{
			name:   "closed on expiry",
			expiry: time.Hour,
		},
		{
			name:   "already expired",
			expiry: -time.Hour,
		},
	}

	// Assert that the connections remain open.
	connectionOpenAssertion := func(t *testing.T, conn *fakeConn) {
		assert.False(t, conn.closed.Load())
	}

	// Assert that the connections are eventually closed.
	connectionClosedAssertion := func(t *testing.T, conn *fakeConn) {
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			assert.True(t, conn.closed.Load())
		}, 5*time.Second, 100*time.Millisecond)
	}

	pref := types.DefaultAuthPreference()
	pref.SetDisconnectExpiredCert(true)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			conn := &fakeConn{}

			var expiry time.Time
			if test.expiry != 0 {
				expiry = clock.Now().Add(test.expiry)
			}
			identity := TestIdentity{
				I: authz.LocalUser{
					Username: "llama",
					Identity: tlsca.Identity{Username: "llama", Expires: expiry},
				},
			}

			creds, err := NewTransportCredentials(TransportCredentialsConfig{
				TransportCredentials: credentials.NewTLS(&tls.Config{}),
				Authorizer:           &fakeAuthorizer{checker: &fakeChecker{}, identity: identity.I},
				UserGetter: fakeUserGetter{
					identity: identity.I,
				},
				Clock:             clock,
				GetAuthPreference: func(ctx context.Context) (types.AuthPreference, error) { return pref, nil },
			})
			require.NoError(t, err, "creating transport credentials")

			validatedConn, _, err := creds.validateIdentity(conn, &credentials.TLSInfo{State: tls.ConnectionState{}})
			switch {
			case test.expiry == 0:
				require.NoError(t, err)
				require.NotNil(t, validatedConn)

				connectionOpenAssertion(t, conn)
				clock.Advance(time.Hour)
				connectionOpenAssertion(t, conn)
			case test.expiry < 0:
				require.NoError(t, err)
				require.NotNil(t, validatedConn)

				connectionClosedAssertion(t, conn)
			default:
				require.NoError(t, err)
				require.NotNil(t, validatedConn)

				connectionOpenAssertion(t, conn)
				clock.BlockUntil(1)
				clock.Advance(test.expiry)
				connectionClosedAssertion(t, conn)
			}
		})
	}
}

type fakeChecker struct {
	services.AccessChecker
	maxConnections    int64
	disconnectExpired *bool
}

func (c *fakeChecker) MaxConnections() int64 {
	return c.maxConnections
}

func (c *fakeChecker) AdjustDisconnectExpiredCert(b bool) bool {
	if c.disconnectExpired == nil {
		return b
	}
	return *c.disconnectExpired
}

type fakeAuthorizer struct {
	authorizeError error
	checker        services.AccessChecker
	identity       authz.IdentityGetter
}

func (a *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	if a.authorizeError != nil {
		return nil, a.authorizeError
	}

	user, err := types.NewUser("llama")
	if err != nil {
		return nil, err
	}

	identity := a.identity
	if identity == nil {
		identity = TestUser(user.GetName()).I
	}

	return &authz.Context{
		User:     user,
		Checker:  a.checker,
		Identity: identity,
	}, nil
}

type fakeEnforcer struct {
	ctx context.Context
	err error
}

func (e *fakeEnforcer) EnforceConnectionLimits(ctx context.Context, identity ConnectionIdentity, closers ...io.Closer) (context.Context, error) {
	return e.ctx, e.err
}

func newTLSConfig(t *testing.T) *tls.Config {
	cert, err := tls.X509KeyPair(tlsCert, keyPEM)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(tlsCACert))

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}
}

var (
	tlsCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDyzCCArOgAwIBAgIQD3MiJ2Au8PicJpCNFbvcETANBgkqhkiG9w0BAQsFADBe
MRQwEgYDVQQKEwtleGFtcGxlLmNvbTEUMBIGA1UEAxMLZXhhbXBsZS5jb20xMDAu
BgNVBAUTJzIwNTIxNzE3NzMzMTIxNzQ2ODMyNjA5NjAxODEwODc0NTAzMjg1ODAe
Fw0yMTAyMTcyMDI3MjFaFw0yMTAyMTgwODI4MjFaMIGCMRUwEwYDVQQHEwxhY2Nl
c3MtYWRtaW4xCTAHBgNVBAkTADEYMBYGA1UEEQwPeyJsb2dpbnMiOm51bGx9MRUw
EwYDVQQKEwxhY2Nlc3MtYWRtaW4xFTATBgNVBAMTDGFjY2Vzcy1hZG1pbjEWMBQG
BSvODwEHEwtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAM5FFaCeK59lwIthyXgSCMZbHTDxsy66Cbm/XhwFbKQLngyS0oKkHbh06INN
UfTAAEaFlMG0CzdAyGyRSu9FK8BE127kRHBs6hb1pTgy2f6TFkFo/h4WTWW4GQSi
O8Al7A2tuRjc3mAnk71q+kvpQYS7tnkhmFCYE8jKxMtlYG39x4kQ6btll7P9zI6X
Zv5RRrlzqADuwZpEcLYVi0TjITqPbx3rDZT4l+EmslhaoG+xE5Vu+GYXLlvwB9E/
amfN1Z9Kps4Ob6Jxxse9kjeMir9mwiNkBWVyhH/LETDA9Xa6sTQ2e75MYM7yXJLY
OmBKV4g176Qf1T1ye7a/Ggn4t2UCAwEAAaNgMF4wDgYDVR0PAQH/BAQDAgWgMB0G
A1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB8GA1Ud
IwQYMBaAFJWqMooE05nf263F341pOO+mPMSqMA0GCSqGSIb3DQEBCwUAA4IBAQCK
s0yPzkSuCY/LFeHJoJeNJ1SR+EKbk4zoAnD0nbbIsd2quyYIiojshlfehhuZE+8P
bzpUNG2aYKq+8lb0NO+OdZW7kBEDWq7ZwC8OG8oMDrX385fLcicm7GfbGCmZ6286
m1gfG9yqEte7pxv3yWM+7X2bzEjCBds4feahuKPNxOAOSfLUZiTpmOVlRzrpRIhu
2XxiuH+E8n4AP8jf/9bGvKd8PyHohtHVf8HWuKLZxWznQhoKkcfmUmlz5q8ci4Bq
WQdM2NXAMABGAofGrVklPIiraUoHzr0Xxpia4vQwRewYXv8bCPHW+8g8vGBGvoG2
gtLit9DL5DR5ac/CRGJt
-----END CERTIFICATE-----`)

	keyPEM = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAzkUVoJ4rn2XAi2HJeBIIxlsdMPGzLroJub9eHAVspAueDJLS
gqQduHTog01R9MAARoWUwbQLN0DIbJFK70UrwETXbuREcGzqFvWlODLZ/pMWQWj+
HhZNZbgZBKI7wCXsDa25GNzeYCeTvWr6S+lBhLu2eSGYUJgTyMrEy2Vgbf3HiRDp
u2WXs/3Mjpdm/lFGuXOoAO7BmkRwthWLROMhOo9vHesNlPiX4SayWFqgb7ETlW74
ZhcuW/AH0T9qZ83Vn0qmzg5vonHGx72SN4yKv2bCI2QFZXKEf8sRMMD1drqxNDZ7
vkxgzvJcktg6YEpXiDXvpB/VPXJ7tr8aCfi3ZQIDAQABAoIBAE1Vk207wAksAgt/
5yQwRr/vizs9czuSnnDYsbT5x6idfm0iYvB+DXKJyl7oD1Ee5zuJe6NAGHBnxn0F
4D1jBqs4ZDj8NjicbQucn4w5bIfIp7BwZ83p+KypYB/fn11EGoNqXZpXvLv6Oqbq
w9rQIjNcmWZC1TNqQQioFS5Y3NV/gw5uYCRXZlSLMsRCvcX2+LN2EP76ZbkpIVpT
CidC2TxwFPPbyMsG774Olfz4U2IDgX1mO+milF7RIa/vPADSeHAX6tJHmZ13GsyP
0GAdPbFa0Ls/uykeGi1uGPFkdkNEqbWlDf1Z9IG0dr/ck2eh8G2X8E+VFgzsKp4k
WtH9nGECgYEA53lFodLiKQjQR7IoUmGp+P6qnrDwOdU1RfT9jse35xOb9tYvZs3X
kUXU+MEGAMW1Pvmo1v9xOjZbdFYB9I/tIYTSyjYQNaFjgJMPMLSx2qjMzhFXAY5f
8t20/CBt2V1q46aa8tR2ll//QvY4mqvJUaaB0pkuasFbKMXJcGKdvdkCgYEA5CAo
UI8NVA9GqAJfs7hkGHQwpX1X1+JpFhF4dZKsV40NReqaK0vd/mWTYjlMOPO6oolr
PoCDUlQYU6poIDtEnfJ6KkYuLMgxZKnS2OlDthKoZJe9aUTCP1RhTVHyyABRXbGg
tNMKFYkZ38C9+JM+X5T0eKZTHeK+wjiZd55+sm0CgYAmyp0PxI6gP9jf2wyE2dcp
YkxnsdFgb8mwwqDnl7LLJ+8gS76/5Mk2kFRjp72AzaFVP3O7LC3miouDEJLdUG12
C5NjzfGjezt4payLBg00Tsub0S4alaigw+T7x9eA8PXj1tzqyw5gnw/hQfA0g4uG
gngJOiCcRXEogRUEH5K96QKBgFUnB8ViUHhTJ22pTS3Zo0tZe5saWYLVGaLKLKu+
byRTG2RAuQF2VUwTgFtGxgPwPndTUjvHXr2JdHcugaWeWfOXQjCrd6rxozZPCcw7
7jF1b3P1DBfSOavIBHYHI9ex/q05k6JLsFTvkz/pQ0AZPkwRXtv2QcpDDC+VTvvO
pr5VAoGBAJBhNjs9wAu+ZoPcMZcjIXT/BAj2tQYiHoRnNpvQjDYbQueUBeI0Ry8d
5QnKS2k9D278P6BiDBz1c+fS8UErOxY6CS0pi4x3fjMliPwXj/w7AzjlXgDBhRcp
90Ns/9SamlBo9j8ETm9g9D3EVir9zF5XvoR13OdN9gabGy1GuubT
-----END RSA PRIVATE KEY-----`)

	tlsCACert = []byte(`-----BEGIN CERTIFICATE-----
MIIDiTCCAnGgAwIBAgIRAJlp/39yg8U604bjsxgcoC0wDQYJKoZIhvcNAQELBQAw
XjEUMBIGA1UEChMLZXhhbXBsZS5jb20xFDASBgNVBAMTC2V4YW1wbGUuY29tMTAw
LgYDVQQFEycyMDM5MjIyNTY2MzcxMDQ0NDc3MzYxNjA0MTk0NjU2MTgzMDA5NzMw
HhcNMjEwMjAzMDAyOTQ2WhcNMzEwMjAxMDAyOTQ2WjBeMRQwEgYDVQQKEwtleGFt
cGxlLmNvbTEUMBIGA1UEAxMLZXhhbXBsZS5jb20xMDAuBgNVBAUTJzIwMzkyMjI1
NjYzNzEwNDQ0NzczNjE2MDQxOTQ2NTYxODMwMDk3MzCCASIwDQYJKoZIhvcNAQEB
BQADggEPADCCAQoCggEBAKnIJmcKgzj/FbvF6/OYkw3owsS3XU6AcJZ7HmTfYpZF
ozqTDVJdHMFQVfu6cp/6hkzoZ/t7hKT6Nd/O2mlIZdBCfT5ZKESRvTGAeCUANKA5
/D4+6PDdW6AutOFUGbHQ1nYLB7HRgaXF/aZmzFPsPNwX8Wm8EByL+Dws61EmSBBv
Soado5rPG78mAnRpFvyYbzBDkxzsgLIfv0EPw9jhSjrT3OVjCXnBv53u2S+UbJfR
jmI7MutjNbJ/rIBp7JpRHJASmW7oj65WPH0SE0+67XwXYKbs0b7CcSuYW+1S+l9R
uGswW4hqwMloP9sTZoWzgT+nCXQSYUavQF+UJZ/dklMCAwEAAaNCMEAwDgYDVR0P
AQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFC4555Otcq4GAYcc
QQDJgh1TKrFvMA0GCSqGSIb3DQEBCwUAA4IBAQBYsEMJYmSD6Dc1suUEnkWo7kOw
va/aaOu0Phy9SK3hCjg+tatHVVDHO2dZdVCAvCe36BcLiZL1ovFZAXzEzOovwLx/
AVjXpMXTJj52RSMOAtRVSkk3/WOHrGOGIBW2bCKxF4ORXJfWJrdtaObwPPV5sbDC
ACdlNMujdBfUM8EDNmvREI/sVmqL6FK9l6elO/bWLJoiaRTxI+CMixpfIYq8pAwJ
UpgZGjcwco4eqXm7rgbQ4wLaMU6hyk8OE5Glk5E6qpnbVzlrL/jl2iE6EqvI6GJn
Na6B0YR7mdrrL+lyzymnOr6UOrT5nUWRAB1QeY7dhBNnsvoZwaS3VLSc1KCk
-----END CERTIFICATE-----`)
)
