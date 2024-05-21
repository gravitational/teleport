/*
Copyright 2015-2022 Gravitational, Inc.

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

package main

import (
	"crypto/x509"
	"encoding/pem"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGenerateClientCertFile(t *testing.T) {
	td := t.TempDir()
	cp := "client.crt"
	kp := "client.key"

	// Generate certs in memory
	certs, err := GenerateMTLSCerts([]string{"localhost"}, nil, time.Second, 1024)
	require.NoError(t, err)
	require.NotNil(t, certs.caCert.Issuer)
	require.NotNil(t, certs.clientCert.Issuer)
	require.NotNil(t, certs.serverCert.Issuer)
	// don't be self-signed
	require.NotEqual(t, certs.serverCert.Issuer, certs.serverCert.Subject)
	require.NotEqual(t, certs.clientCert.Issuer, certs.clientCert.Subject)
	// leaf certs should have SANs
	require.NotEmpty(t, certs.serverCert.DNSNames)
	require.NotEmpty(t, certs.clientCert.DNSNames)
	// leaf certs should be issued by the CA
	require.Equal(t, certs.serverCert.Issuer, certs.caCert.Subject)
	require.Equal(t, certs.clientCert.Issuer, certs.caCert.Subject)

	// make sure leaf certs are not CA certs
	require.NotEqual(t, certs.caCert.Subject.CommonName, certs.clientCert.Subject.CommonName, "CA and client certs should be different")
	require.NotEqual(t, certs.caCert.Subject.CommonName, certs.serverCert.Subject.CommonName, "CA and server certs should be different")

	// leaf certs should have SAN
	require.NotEmpty(t, certs.serverCert.DNSNames)
	require.NotEmpty(t, certs.clientCert.DNSNames)
	// server leaf cert should have SAN DNS:localhost
	require.Equal(t, "localhost", certs.serverCert.DNSNames[0])

	// Write the cert to the tempdir
	err = certs.ClientCert.WriteFile(path.Join(td, cp), path.Join(td, kp), ".")
	require.NoError(t, err)
	f, err := os.Open(path.Join(td, cp))
	require.NoError(t, err)
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	der, _ := pem.Decode(b)
	rc, err := x509.ParseCertificate(der.Bytes)
	require.NoError(t, err)
	require.Equal(t, certs.clientCert.Issuer.CommonName, rc.Issuer.CommonName)
}
