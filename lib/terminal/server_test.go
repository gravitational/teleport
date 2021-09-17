// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package terminal_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/terminal"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	terminalpb "github.com/gravitational/teleport/protogen/teleport/terminal/v1"
)

func TestStart(t *testing.T) {
	tests := []struct {
		name string
		opts terminal.ServerOpts
	}{
		{
			name: "TCP socket",
			opts: terminal.ServerOpts{
				Addr: "localhost:",
			},
		},
		{
			name: "Unix socket",
			opts: terminal.ServerOpts{
				Addr: fmt.Sprintf("unix://%v/terminal.sock", t.TempDir()),
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			server, err := terminal.Start(ctx, test.opts)
			require.NoError(t, err)
			defer func() {
				cancel() // Stop the server.
				require.NoError(t, <-server.C)
				require.NoError(t, <-server.C) // subsequent calls are allowed
			}()
			require.NotEmpty(t, server.Addr)                  // Addr present
			require.Contains(t, server.Addr, server.DialAddr) // Addr >= DialAddr

			cc, err := grpc.Dial(server.DialAddr, grpc.WithInsecure())
			require.NoError(t, err)
			defer cc.Close()

			term := terminalpb.NewTerminalServiceClient(cc)
			_, err = term.ListClusters(ctx, &terminalpb.ListClustersRequest{})
			if got := status.Code(err); got != codes.Unimplemented {
				t.Errorf("ListClusters returned err = %v, want %s", err, codes.Unimplemented)
			}
		})
	}
}

func TestStart_configJSON(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Stops the server.

	cfgOut := &bytes.Buffer{}
	server, err := terminal.Start(ctx, terminal.ServerOpts{
		Addr:          "badaddr",
		ReadFromInput: true,
		ConfigInput:   strings.NewReader(`{"addr": "localhost:"}`),
		ConfigOutput:  cfgOut,
	})
	require.NoError(t, err)

	decodedOpts := &terminal.RuntimeOpts{}
	require.NoError(t, json.NewDecoder(cfgOut).Decode(decodedOpts))
	require.Equal(t, server.Addr, decodedOpts.Addr)
}

func TestStart_tls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Stops the server.

	serverCert, serverKey := makeSelfSigned(t, &x509.Certificate{
		Subject:     pkix.Name{CommonName: "localhost"},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	})
	clientCert, clientKey := makeSelfSigned(t, &x509.Certificate{
		Subject:        pkix.Name{CommonName: "llama"},
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		EmailAddresses: []string{"llama@goteleport.com"},
	})

	server, err := terminal.Start(ctx, terminal.ServerOpts{
		Addr:      "localhost:",
		CertFile:  string(serverCert),
		KeyFile:   string(serverKey),
		ClientCAs: []string{string(clientCert)},
	})
	require.NoError(t, err)
	require.True(t, server.TLS, "TLS enabled")
	require.True(t, server.MTLS, "mTLS enabled")

	clientPair, err := tls.X509KeyPair(clientCert, clientKey)
	require.NoError(t, err)

	rootCAs := x509.NewCertPool()
	require.True(t, rootCAs.AppendCertsFromPEM(serverCert), "append server cert to root CAs")

	cc, err := grpc.Dial(server.DialAddr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientPair},
		RootCAs:      rootCAs,
		MinVersion:   tls.VersionTLS13,
	})))
	require.NoError(t, err)

	term := terminalpb.NewTerminalServiceClient(cc)
	_, err = term.ListClusters(ctx, &terminalpb.ListClustersRequest{})
	if got := status.Code(err); got != codes.Unimplemented {
		t.Errorf("ListClusters returned err = %v, want %s", err, codes.Unimplemented)
	}
}

func makeSelfSigned(t *testing.T, template *x509.Certificate) (cert []byte, key []byte) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	ecKeyBytes, err := x509.MarshalECPrivateKey(ecKey)
	require.NoError(t, err)
	key = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: ecKeyBytes,
	})

	// Set common template fields.
	sn, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	require.NoError(t, err)
	template.SerialNumber = sn
	template.NotBefore = time.Now().Add(-1 * time.Minute)
	template.NotAfter = time.Now().Add(60 * time.Minute)
	template.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
	template.BasicConstraintsValid = true
	template.IsCA = true

	rawCert, err := x509.CreateCertificate(rand.Reader, template, template, &ecKey.PublicKey, ecKey)
	require.NoError(t, err)
	cert = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCert,
	})
	return cert, key
}
