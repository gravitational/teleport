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

package teleterm

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/utils/cert"
)

const (
	// tshdCertFileName is the file name of the cert created by the tshd process. The Electron app
	// expects it to exist under this name in the certs dir passed through a flag to tshd.
	tshdCertFileName = "tshd.crt"
	// rendererCertFileName is the file name of the cert created by the renderer process of the
	// Electron app.
	rendererCertFileName = "renderer.crt"
	// mainProcessCertFileName is the file name of the cert created by the main process of the
	// Electron app.
	mainProcessCertFileName = "main-process.crt"
)

// createServerCredentials creates mTLS credentials for a gRPC server. This mTLS setup hinges on
// server and client processes being able to read and write files to a directory that only the
// current user is able to write to. It's meant a simple replacement for using Unix sockets in
// environments like Windows where we can't use them.
//
// In this mTLS setup, there is no single CA which creates certs for clients. Instead, each client
// generates a self-signed cert that it's going to use to initiate a connection. The cert is then
// saved under a predetermined path. The path is passed out of bound during tsh daemon startup.
//
// createServerCredentials then reads each cert and adds it to ClientCAs. Those certs will be then
// used by clients to initiate connections.
//
// The startup of Connect is orchestrated in a way where those client public keys are expected to be saved
// to disk before a client attempts to connect to the server.
func createServerCredentials(serverKeyPair tls.Certificate, clientCertPaths []string) (grpc.ServerOption, error) {
	config := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{serverKeyPair},
	}

	config.GetConfigForClient = func(info *tls.ClientHelloInfo) (*tls.Config, error) {
		certPool := x509.NewCertPool()

		for _, clientCertPath := range clientCertPaths {
			log := slog.With("cert_path", clientCertPath)

			clientCert, err := os.ReadFile(clientCertPath)
			if err != nil {
				log.ErrorContext(info.Context(), "Failed to read the client cert file", "error", err)
				// Fall back to the default config.
				return nil, nil
			}

			if !certPool.AppendCertsFromPEM(clientCert) {
				log.ErrorContext(info.Context(), "Failed to add the client cert to the pool")
				// Fall back to the default config.
				return nil, nil
			}
		}

		configClone := config.Clone()
		configClone.ClientCAs = certPool

		return configClone, nil
	}

	return grpc.Creds(credentials.NewTLS(config)), nil
}

// createClientCredentials creates mTLS credentials for a gRPC client. The server cert file is read
// upfront as there is no way to lazily add RootCAs to a tls.Config.
func createClientCredentials(clientKeyPair tls.Certificate, serverCertPath string) (grpc.DialOption, error) {
	config, err := createClientTLSConfig(clientKeyPair, serverCertPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return grpc.WithTransportCredentials(credentials.NewTLS(config)), nil
}

func createClientTLSConfig(clientKeyPair tls.Certificate, serverCertPath string) (*tls.Config, error) {
	serverCert, err := os.ReadFile(serverCertPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read the server cert file")
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(serverCert) {
		return nil, trace.BadParameter("failed to add server cert to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientKeyPair},
		RootCAs:      certPool,
	}, nil
}

func generateAndSaveCert(targetPath string, eku ...x509.ExtKeyUsage) (tls.Certificate, error) {
	// The cert is first saved under a temp path and then renamed to targetPath. This prevents other
	// processes from reading a half-written file.
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), filepath.Base(targetPath))
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	defer os.Remove(tempFile.Name())

	cert, err := cert.GenerateSelfSignedCert([]string{"localhost"}, nil, eku...)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err, "failed to generate the certificate")
	}

	if err = tempFile.Chmod(0600); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	if _, err = tempFile.Write(cert.Cert); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	if err = tempFile.Close(); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	if err = os.Rename(tempFile.Name(), targetPath); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	certificate, err := keys.X509KeyPair(cert.Cert, cert.PrivateKey)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return certificate, nil
}
