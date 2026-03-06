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

package app

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// loadMTLSConfig loads mTLS configuration from disk.
func loadMTLSConfig(cipherSuites []uint16, appTLS types.AppTLS) (*tls.Config, error) {
	switch appTLS.Mode {
	case types.AppTLS_MODE_VERIFY_CA:
		return mtlsConfigVerifyCA(cipherSuites, appTLS)
	case types.AppTLS_MODE_INSECURE:
		return mtlsConfigInsecure(cipherSuites, appTLS)
	default:
		return mtlsConfigFull(cipherSuites, appTLS)
	}
}

func mtlsConfigFull(cipherSuites []uint16, appTLS types.AppTLS) (*tls.Config, error) {
	tlsConfig := utils.TLSConfig(cipherSuites)
	clientCert, err := tls.LoadX509KeyPair(appTLS.CertPath, appTLS.KeyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := os.ReadFile(appTLS.CaPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(ca); !ok {
		return nil, trace.BadParameter("unable to load provided CA")
	}

	tlsConfig.Certificates = []tls.Certificate{clientCert}
	tlsConfig.RootCAs = caPool
	return tlsConfig, nil
}

func mtlsConfigVerifyCA(cipherSuites []uint16, appTLS types.AppTLS) (*tls.Config, error) {
	tlsConfig, err := mtlsConfigFull(cipherSuites, appTLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Base on https://github.com/golang/go/blob/master/src/crypto/tls/example_test.go#L193-L208
	// Set InsecureSkipVerify to skip the default validation we are
	// replacing. This will not disable VerifyConnection.
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = utils.VerifyConnectionIgnoreServerName(time.Now, func() (*x509.CertPool, error) {
		return tlsConfig.RootCAs, nil
	})
	// ServerName is irrelevant in this case. Set it to default value to make
	// it explicit.
	tlsConfig.ServerName = ""
	return tlsConfig, nil
}

func mtlsConfigInsecure(cipherSuites []uint16, appTLS types.AppTLS) (*tls.Config, error) {
	tlsConfig, err := mtlsConfigFull(cipherSuites, appTLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Accept any certificate provided by upstream server.
	tlsConfig.InsecureSkipVerify = true
	// Remove certificate validation if set.
	tlsConfig.VerifyConnection = nil

	return tlsConfig, nil
}
