// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

// We use x509usefallbackroots to replace the system cert pool with a system
// cert pool with an extra CA.
//go:debug x509usefallbackroots=1

package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"runtime"
	"time"

	"github.com/gravitational/teleport/api/cryptopatch"
)

// webpkiCACert and webpkiCAKey are a CA added to the system cert pool.
var (
	webpkiCACert *x509.Certificate
	webpkiCAKey  *ecdsa.PrivateKey
)

func init() {
	// we can't add to the system cert pool on platforms with a system verifier,
	// because verifying against the default cert pool only uses the system
	// verifier, so on those we just don't set up our additional CA
	//
	// TODO(espadolini): see if embedding the mozilla trust store could work
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		return
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		panic(err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      pkix.Name{CommonName: "Teleport testing web PKI CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour * 24 * 365),

		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth,
		},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	key, err := cryptopatch.GenerateECDSAKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		panic(err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	pool.AddCert(cert)
	x509.SetFallbackRoots(pool)

	webpkiCACert, webpkiCAKey = cert, key
}
