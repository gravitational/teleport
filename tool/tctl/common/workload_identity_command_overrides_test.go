// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apiconstants "github.com/gravitational/teleport/api/constants"
	trustv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

func TestOverrideCreate(t *testing.T) {
	d := t.TempDir()

	c1a := newSelfSignedCert(t, "c1a")
	c1b := newSelfSignedCert(t, "c1b")
	c1Path := filepath.Join(d, "c1.pem")
	require.NoError(t, os.WriteFile(
		c1Path,
		bytes.Join([][]byte{
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c1a.Leaf.Raw}),
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c1b.Leaf.Raw}),
		}, nil),
		0o600,
	))

	c2a := newSelfSignedCert(t, "c2a")
	c2Path := filepath.Join(d, "c2.pem")
	require.NoError(t, os.WriteFile(
		c2Path,
		bytes.Join([][]byte{
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c2a.Leaf.Raw}),
		}, nil),
		0o600,
	))

	c3a := newSelfSignedCert(t, "c3a")
	c3Path := filepath.Join(d, "c3.pem")
	require.NoError(t, os.WriteFile(
		c3Path,
		bytes.Join([][]byte{
			pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c3a.Leaf.Raw}),
		}, nil),
		0o600,
	))

	overrideServer := &overrideServer{}
	clt := runFakeAPIServer(t, func(s *grpc.Server) {
		proto.RegisterAuthServiceServer(s, &domainNameServer{
			domainName: "clustername",
		})
		trustv1.RegisterTrustServiceServer(s, &caServer{
			cas: map[types.CertAuthID]*types.CertAuthorityV2{
				{Type: types.SPIFFECA, DomainName: "clustername"}: {
					Spec: types.CertAuthoritySpecV2{
						ActiveKeys: types.CAKeySet{
							TLS: []*types.TLSKeyPair{
								{Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c1a.Leaf.Raw})},
								{Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c2a.Leaf.Raw})},
							},
						},
					},
				},
			},
		})
		workloadidentityv1.RegisterX509OverridesServiceServer(s, overrideServer)
	})

	err := runCommand(t, clt, &WorkloadIdentityCommand{stdout: io.Discard}, []string{
		"workload-identity", "x509-issuer-overrides", "create", c1Path,
	})
	require.ErrorContains(t, err, "expected 2 override(s), got 1")

	// c3 is not in the SPIFFE CA
	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: io.Discard}, []string{
		"workload-identity", "x509-issuer-overrides", "create", c1Path, c3Path,
	})
	require.ErrorContains(t, err, " does not match any issuer")

	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: io.Discard}, []string{
		"workload-identity", "x509-issuer-overrides", "create", c1Path, c2Path,
	})
	require.NoError(t, err)
	require.Equal(t, 1, overrideServer.createCalls)
	require.Equal(t, 0, overrideServer.upsertCalls)
	overrideServer.createCalls = 0
	require.Equal(t, "default", overrideServer.def.GetMetadata().GetName())

	require.Len(t, overrideServer.def.GetSpec().GetOverrides(), 2)
	o := overrideServer.def.GetSpec().GetOverrides()
	o1, o2 := o[0], o[1]
	if len(o1.GetChain()) > len(o2.GetChain()) {
		o1, o2 = o2, o1
	}

	require.Equal(t, c2a.Leaf.Raw, o1.GetIssuer())
	require.Len(t, o1.GetChain(), 1)
	require.Equal(t, c2a.Leaf.Raw, o1.GetChain()[0])

	require.Equal(t, c1a.Leaf.Raw, o2.GetIssuer())
	require.Len(t, o2.GetChain(), 2)
	require.Equal(t, c1a.Leaf.Raw, o2.GetChain()[0])
	require.Equal(t, c1b.Leaf.Raw, o2.GetChain()[1])

	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: io.Discard}, []string{
		"workload-identity", "x509-issuer-overrides", "create", c1Path, c2Path,
	})
	require.ErrorAs(t, err, new(*trace.AlreadyExistsError))
	require.Equal(t, 1, overrideServer.createCalls)
	require.Equal(t, 0, overrideServer.upsertCalls)

	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: io.Discard}, []string{
		"workload-identity", "x509-issuer-overrides", "create", "--force", c1Path, c2Path,
	})
	require.NoError(t, err)
	require.Equal(t, 1, overrideServer.createCalls)
	require.Equal(t, 1, overrideServer.upsertCalls)
}

func TestOverrideSign(t *testing.T) {
	c1 := newSelfSignedCert(t, "c1")
	c2 := newSelfSignedCert(t, "c2")

	csr1, err := x509.CreateCertificateRequest(
		rand.Reader,
		&x509.CertificateRequest{Subject: pkix.Name{CommonName: "csr1"}},
		c1.PrivateKey,
	)
	require.NoError(t, err)
	csr2, err := x509.CreateCertificateRequest(
		rand.Reader,
		&x509.CertificateRequest{Subject: pkix.Name{CommonName: "csr2"}},
		c2.PrivateKey,
	)
	require.NoError(t, err)

	clt := runFakeAPIServer(t, func(s *grpc.Server) {
		proto.RegisterAuthServiceServer(s, &domainNameServer{
			domainName: "clustername",
		})
		trustv1.RegisterTrustServiceServer(s, &caServer{
			cas: map[types.CertAuthID]*types.CertAuthorityV2{
				{Type: types.SPIFFECA, DomainName: "clustername"}: {
					Spec: types.CertAuthoritySpecV2{
						ActiveKeys: types.CAKeySet{
							TLS: []*types.TLSKeyPair{
								{Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c1.Leaf.Raw})},
								{Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c2.Leaf.Raw})},
							},
						},
					},
				},
			},
		})
		workloadidentityv1.RegisterX509OverridesServiceServer(s, &csrServer{
			resp: map[string][]byte{
				string(c1.Leaf.Raw): csr1,
				string(c2.Leaf.Raw): csr2,
			},
		})
	})

	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: os.Stdout}, []string{
		"workload-identity", "x509-issuer-overrides", "sign-csrs", "--creation-mode", "0",
	})
	require.ErrorAs(t, err, new(*trace.BadParameterError))

	out := new(bytes.Buffer)
	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: out}, []string{
		"workload-identity", "x509-issuer-overrides", "sign-csrs",
	})
	require.NoError(t, err)

	l, err := out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "CN=c1\n", l)
	b, rest := pem.Decode(out.Bytes())
	require.NotNil(t, b)
	require.Equal(t, "CERTIFICATE REQUEST", b.Type)
	require.Equal(t, csr1, b.Bytes)
	out.Next(out.Len() - len(rest))

	l, err = out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "CN=c2\n", l)
	b, rest = pem.Decode(out.Bytes())
	require.NotNil(t, b)
	require.Equal(t, "CERTIFICATE REQUEST", b.Type)
	require.Equal(t, csr2, b.Bytes)
	out.Next(out.Len() - len(rest))

	require.Zero(t, out.Len())

	clt = runFakeAPIServer(t, func(s *grpc.Server) {
		proto.RegisterAuthServiceServer(s, &domainNameServer{
			domainName: "clustername",
		})
		trustv1.RegisterTrustServiceServer(s, &caServer{
			cas: map[types.CertAuthID]*types.CertAuthorityV2{
				{Type: types.SPIFFECA, DomainName: "clustername"}: {
					Spec: types.CertAuthoritySpecV2{
						ActiveKeys: types.CAKeySet{
							TLS: []*types.TLSKeyPair{
								{Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c1.Leaf.Raw})},
								{Cert: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c2.Leaf.Raw})},
							},
						},
					},
				},
			},
		})
		workloadidentityv1.RegisterX509OverridesServiceServer(s, &csrServer{
			resp: map[string][]byte{
				string(c1.Leaf.Raw): csr1,
			},
		})
	})

	out.Reset()

	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: out}, []string{
		"workload-identity", "x509-issuer-overrides", "sign-csrs",
	})
	require.ErrorAs(t, err, new(*trace.NotFoundError))
	require.Zero(t, out.Len())

	err = runCommand(t, clt, &WorkloadIdentityCommand{stdout: out}, []string{
		"workload-identity", "x509-issuer-overrides", "sign-csrs", "--force",
	})
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	l, err = out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "CN=c1\n", l)
	b, rest = pem.Decode(out.Bytes())
	require.NotNil(t, b)
	require.Equal(t, "CERTIFICATE REQUEST", b.Type)
	require.Equal(t, csr1, b.Bytes)
	out.Next(out.Len() - len(rest))

	l, err = out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "CN=c2\n", l)
	l, err = out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, signKeyNotFoundMessage+"\n", l)

	require.Zero(t, out.Len())
}

type domainNameServer struct {
	proto.UnimplementedAuthServiceServer
	domainName string
}

func (s *domainNameServer) GetDomainName(ctx context.Context, req *emptypb.Empty) (*proto.GetDomainNameResponse, error) {
	return &proto.GetDomainNameResponse{DomainName: s.domainName}, nil
}

type caServer struct {
	trustv1.UnimplementedTrustServiceServer
	cas map[types.CertAuthID]*types.CertAuthorityV2
}

func (s *caServer) GetCertAuthority(ctx context.Context, req *trustv1.GetCertAuthorityRequest) (*types.CertAuthorityV2, error) {
	ca, ok := s.cas[types.CertAuthID{
		Type:       types.CertAuthType(req.Type),
		DomainName: req.Domain,
	}]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "")
	}
	return ca, nil
}

type overrideServer struct {
	workloadidentityv1.UnimplementedX509OverridesServiceServer
	def         *workloadidentityv1.X509IssuerOverride
	createCalls int
	upsertCalls int
}

func (s *overrideServer) CreateX509IssuerOverride(ctx context.Context, req *workloadidentityv1.CreateX509IssuerOverrideRequest) (*workloadidentityv1.X509IssuerOverride, error) {
	s.createCalls++
	if req.GetX509IssuerOverride() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "")
	}
	if s.def != nil {
		return nil, status.Errorf(codes.AlreadyExists, "")
	}
	s.def = req.GetX509IssuerOverride()
	return s.def, nil
}

func (s *overrideServer) UpsertX509IssuerOverride(ctx context.Context, req *workloadidentityv1.UpsertX509IssuerOverrideRequest) (*workloadidentityv1.X509IssuerOverride, error) {
	s.upsertCalls++
	if req.GetX509IssuerOverride() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "")
	}
	s.def = req.GetX509IssuerOverride()
	return s.def, nil
}

type csrServer struct {
	workloadidentityv1.UnimplementedX509OverridesServiceServer
	resp map[string][]byte
}

const signKeyNotFoundMessage = "key not found or something idk"

func (s *csrServer) SignX509IssuerCSR(ctx context.Context, req *workloadidentityv1.SignX509IssuerCSRRequest) (*workloadidentityv1.SignX509IssuerCSRResponse, error) {
	if req.GetCsrCreationMode() == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "")
	}
	csr, ok := s.resp[string(req.GetIssuer())]
	if !ok {
		return nil, status.Errorf(codes.NotFound, signKeyNotFoundMessage)
	}
	return &workloadidentityv1.SignX509IssuerCSRResponse{Csr: csr}, nil
}

func runFakeAPIServer(t *testing.T, register func(*grpc.Server)) *authclient.Client {
	cert := newSelfSignedCert(t, "")
	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)

	srv := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS13,
			NextProtos:   []string{"h2"},
		})),
	)
	t.Cleanup(srv.Stop)

	register(srv)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go srv.Serve(l)

	clt, err := apiclient.New(context.Background(), apiclient.Config{
		Addrs: []string{l.Addr().String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(&tls.Config{
				MinVersion: tls.VersionTLS13,
				NextProtos: []string{"h2"},
				RootCAs:    pool,
			}),
		},
		DialInBackground: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { clt.Close() })

	return &authclient.Client{APIClient: clt}
}

func newSelfSignedCert(t *testing.T, cn string) tls.Certificate {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	sn, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	require.NoError(t, err)

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: sn,
		Subject: pkix.Name{
			CommonName: cn,
		},

		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(2159 * time.Hour),

		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},

		BasicConstraintsValid: true,
		IsCA:                  true,

		DNSNames: []string{
			apiconstants.APIDomain,
		},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}
}
