/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestDefaultCreds(t *testing.T) {

	// DefaultCreds works without any input
	creds, err := DefaultCreds()
	require.NoError(t, err)
	_, err = New(Config{
		Addrs: []string{"proxy.example.com"},
		Creds: creds,
	})
	require.NoError(t, err)

	// DefaultCreds attempts each loader, returning the first success.
	creds, err = DefaultCreds(
		IdentityFileCredsLoader("no-path"),
		CertFilesCredsLoader("no-path"),
		ProfileCredsLoader(""),
	)
	require.NoError(t, err)
	_, err = New(Config{
		Addrs: []string{"proxy.example.com"},
		Creds: creds,
	})
	require.NoError(t, err)

	// Should return error if all loaders fail.
	creds, err = DefaultCreds(
		IdentityFileCredsLoader("no-path"),
		CertFilesCredsLoader("no-path"),
		ProfileCredsLoader("no-path"),
	)
	require.Error(t, err)
	require.Nil(t, creds)

}

func TestIdentityCreds(t *testing.T) {
	expectedTLS, err := getExpectedTLS()
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "file")
	idFile := &IdentityFile{
		PrivateKey: []byte(keyPEM),
		Certs: Certs{
			TLS: []byte(certPEM),
		},
		CACerts: CACerts{
			TLS: [][]byte{[]byte(caCertPEM)},
		},
	}

	// Write identity file
	err = WriteIdentityFile(idFile, path)
	require.NoError(t, err)

	// DefaultCreds attempts each loader, returning the first
	// success, or returning an error if all fail.
	creds, err := IdentityFileCreds(path)
	require.NoError(t, err)
	require.Equal(t, creds.TLS, expectedTLS)

	// Create client from Creds
	_, err = New(Config{
		Addrs: []string{"proxy.example.com"},
		Creds: creds,
	})
	require.NoError(t, err)
}

func TestProfileCreds(t *testing.T) {
	expectedTLS, err := getExpectedTLS()
	require.NoError(t, err)

	dir := t.TempDir()
	p := &Profile{WebProxyAddr: "proxy", Username: "username"}
	p.SaveToDir(dir, true)

	err = writeProfileCerts(p, dir)
	require.NoError(t, err)

	creds, err := ProfileCreds(dir)
	require.NoError(t, err)
	require.Equal(t, creds.TLS, expectedTLS)

	// Create client from Creds
	_, err = New(Config{
		Addrs: []string{"proxy.example.com"},
		Creds: creds,
	})
	require.NoError(t, err)
}

func TestCertsFilesCreds(t *testing.T) {
	expectedTLS, err := getExpectedTLS()
	require.NoError(t, err)

	path := t.TempDir() + "username"
	writeCertsFilesCerts(path)

	creds, err := CertFilesCreds(path)
	require.NoError(t, err)
	require.Equal(t, creds.TLS, expectedTLS)

	// Create client from Creds
	_, err = New(Config{
		Addrs: []string{"proxy.example.com"},
		Creds: creds,
	})
	require.NoError(t, err)
}

func writeProfileCerts(p *Profile, dir string) error {
	// Write creds to the correct path within profile.
	keyDir := filepath.Join(dir, SessionKeyDir)
	os.MkdirAll(keyDir, 0700)

	userKeyDir := filepath.Join(keyDir, p.Name())
	os.MkdirAll(userKeyDir, 0700)

	certPath := filepath.Join(userKeyDir, p.Username+FileExtTLSCert)
	if err := ioutil.WriteFile(certPath, []byte(certPEM), 0600); err != nil {
		return trace.Wrap(err)
	}
	keyPath := filepath.Join(userKeyDir, p.Username)
	if err := ioutil.WriteFile(keyPath, []byte(keyPEM), 0600); err != nil {
		return trace.Wrap(err)
	}
	certsPath := filepath.Join(userKeyDir, FileNameTLSCerts)
	if err := ioutil.WriteFile(certsPath, []byte(caCertPEM), 0600); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func writeCertsFilesCerts(path string) error {
	// Write creds to the correct path within profile.
	if err := ioutil.WriteFile(path+".crt", []byte(certPEM), 0600); err != nil {
		return trace.Wrap(err)
	}
	if err := ioutil.WriteFile(path+".key", []byte(keyPEM), 0600); err != nil {
		return trace.Wrap(err)
	}
	if err := ioutil.WriteFile(path+".cas", []byte(caCertPEM), 0600); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getExpectedTLS() (*tls.Config, error) {
	// set expectedTLS from PEM.
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caCertPEM)) {
		return nil, trace.Errorf("invalid CA cert PEM")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

var certPEM = `-----BEGIN CERTIFICATE-----
MIIB0zCCAX2gAwIBAgIJAI/M7BYjwB+uMA0GCSqGSIb3DQEBBQUAMEUxCzAJBgNV
BAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBX
aWRnaXRzIFB0eSBMdGQwHhcNMTIwOTEyMjE1MjAyWhcNMTUwOTEyMjE1MjAyWjBF
MQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50
ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANLJ
hPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wok/4xIA+ui35/MmNa
rtNuC+BdZ1tMuVCPFZcCAwEAAaNQME4wHQYDVR0OBBYEFJvKs8RfJaXTH08W+SGv
zQyKn0H8MB8GA1UdIwQYMBaAFJvKs8RfJaXTH08W+SGvzQyKn0H8MAwGA1UdEwQF
MAMBAf8wDQYJKoZIhvcNAQEFBQADQQBJlffJHybjDGxRMqaRmDhX0+6v02TUKZsW
r5QuVbpQhH6u+0UgcW0jp9QwpxoPTLTWGXEWBBBurxFwiCBhkQ+V
-----END CERTIFICATE-----`

var keyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBANLJhPHhITqQbPklG3ibCVxwGMRfp/v4XqhfdQHdcVfHap6NQ5Wo
k/4xIA+ui35/MmNartNuC+BdZ1tMuVCPFZcCAwEAAQJAEJ2N+zsR0Xn8/Q6twa4G
6OB1M1WO+k+ztnX/1SvNeWu8D6GImtupLTYgjZcHufykj09jiHmjHx8u8ZZB/o1N
MQIhAPW+eyZo7ay3lMz1V01WVjNKK9QSn1MJlb06h/LuYv9FAiEA25WPedKgVyCW
SmUwbPw8fnTcpqDWE3yTO3vKcebqMSsCIBF3UmVue8YU3jybC3NxuXq3wNm34R8T
xVLHwDXh/6NJAiEAl2oHGGLz64BuAfjKrqwz7qMYr9HCLIe/YsoWq/olzScCIQDi
D2lWusoe2/nEqfDVVWGWlyJ7yOmqaVm/iNUN9B2N2g==
-----END RSA PRIVATE KEY-----`

var caCertPEM = `-----BEGIN CERTIFICATE-----
MIIB/jCCAWICCQDscdUxw16XFDAJBgcqhkjOPQQBMEUxCzAJBgNVBAYTAkFVMRMw
EQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBXaWRnaXRzIFB0
eSBMdGQwHhcNMTIxMTE0MTI0MDQ4WhcNMTUxMTE0MTI0MDQ4WjBFMQswCQYDVQQG
EwJBVTETMBEGA1UECBMKU29tZS1TdGF0ZTEhMB8GA1UEChMYSW50ZXJuZXQgV2lk
Z2l0cyBQdHkgTHRkMIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBY9+my9OoeSUR
lDQdV/x8LsOuLilthhiS1Tz4aGDHIPwC1mlvnf7fg5lecYpMCrLLhauAc1UJXcgl
01xoLuzgtAEAgv2P/jgytzRSpUYvgLBt1UA0leLYBy6mQQbrNEuqT3INapKIcUv8
XxYP0xMEUksLPq6Ca+CRSqTtrd/23uTnapkwCQYHKoZIzj0EAQOBigAwgYYCQXJo
A7Sl2nLVf+4Iu/tAX/IF4MavARKC4PPHK3zfuGfPR3oCCcsAoz3kAzOeijvd0iXb
H5jBImIxPL4WxQNiBTexAkF8D1EtpYuWdlVQ80/h/f4pBcGiXPqX5h2PQSQY7hP1
+jwM1FGS4fREIOvlBYr/SzzQRtwrvrzGYxDEDbsC0ZGRnA==
-----END CERTIFICATE-----`
