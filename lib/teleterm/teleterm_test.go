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
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

const (
	// timeout used for most operations in tests.
	timeout = 10 * time.Second
)

type createClientTLSConfigFunc func(t *testing.T, certsDir string) *tls.Config
type connReadExpectationFunc func(t *testing.T, connReadErr error)

func TestStart(t *testing.T) {
	t.Parallel()

	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "teleterm.sock")

	tests := []struct {
		name                    string
		addr                    string
		connReadExpectationFunc connReadExpectationFunc
		// createClientTLSConfigFunc needs to be executed after the server is started. Starting the
		// server saves the public key of the server to disk. Without this key we wouldn't be able to
		// create a valid TLS config for the client.
		//
		// Called only when the server listens on a TCP address.
		createClientTLSConfigFunc createClientTLSConfigFunc
	}{
		{
			// No mTLS.
			name: "unix",
			addr: fmt.Sprintf("unix://%v", sockPath),
			connReadExpectationFunc: func(t *testing.T, connReadErr error) {
				require.NoError(t, connReadErr)
			},
		},
		{
			name: "tcp with valid client cert",
			addr: "tcp://localhost:0",
			createClientTLSConfigFunc: func(t *testing.T, certsDir string) *tls.Config {
				return createValidClientTLSConfig(t, certsDir)
			},
			connReadExpectationFunc: func(t *testing.T, connReadErr error) {
				require.NoError(t, connReadErr)
			},
		},
		{
			// The server reads the client cert from a predetermined path on disk and fall backs to a
			// default config if the cert is not present.
			name: "tcp with client cert not saved to disk",
			addr: "tcp://localhost:0",
			createClientTLSConfigFunc: func(t *testing.T, certsDir string) *tls.Config {
				return &tls.Config{InsecureSkipVerify: true}
			},
			connReadExpectationFunc: func(t *testing.T, connReadErr error) {
				require.ErrorContains(t, connReadErr, "tls:")
			},
		},
		{
			name: "tcp with client cert saved to disk but not provided to server",
			addr: "tcp://localhost:0",
			createClientTLSConfigFunc: func(t *testing.T, certsDir string) *tls.Config {
				createValidClientTLSConfig(t, certsDir)
				return &tls.Config{InsecureSkipVerify: true}
			},
			connReadExpectationFunc: func(t *testing.T, connReadErr error) {
				require.ErrorContains(t, connReadErr, "tls:")
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			homeDir := t.TempDir()
			certsDir := t.TempDir()
			listeningC := make(chan utils.NetAddr)

			cfg := Config{
				Addr:           test.addr,
				HomeDir:        homeDir,
				CertsDir:       certsDir,
				PrehogAddr:     "https://prehog:9999",
				ListeningC:     listeningC,
				KubeconfigsDir: t.TempDir(),
				AgentsDir:      t.TempDir(),
				InstallationID: "foo",
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			serveErr := make(chan error)
			go func() {
				err := Serve(ctx, cfg)
				serveErr <- err
			}()

			select {
			case addr := <-listeningC:
				// Verify that the server accepts connections on the advertised address.
				blockUntilServerAcceptsConnections(t, addr, certsDir,
					test.createClientTLSConfigFunc, test.connReadExpectationFunc)

				// Stop the server.
				cancel()
				require.NoError(t, <-serveErr)
			case <-time.After(timeout):
				t.Fatal("listeningC didn't advertise the address within the timeout")
			case err := <-serveErr:
				t.Fatalf("teleterm.Serve returned sooner than expected, err: %#v", err)
			}
		})
	}

}

// blockUntilServerAcceptsConnections dials the addr and then reads from the connection.
// In case of a unix addr, it waits for the socket file to be created before attempting to dial.
// In case of a tcp addr, it sets up an mTLS config for the dialer.
func blockUntilServerAcceptsConnections(t *testing.T, addr utils.NetAddr, certsDir string,
	createClientTLSConfigFunc createClientTLSConfigFunc, connReadExpectation connReadExpectationFunc) {
	var conn net.Conn
	switch addr.AddrNetwork {
	case "unix":
		conn = dialUnix(t, addr)
	case "tcp":
		conn = dialTCP(t, addr, certsDir, createClientTLSConfigFunc)
	default:
		t.Fatalf("Unknown addr network %v", addr.AddrNetwork)
	}

	t.Cleanup(func() { conn.Close() })

	err := conn.SetReadDeadline(time.Now().Add(timeout))
	require.NoError(t, err)

	out := make([]byte, 1024)
	_, err = conn.Read(out)
	connReadExpectation(t, err)

	err = conn.Close()
	require.NoError(t, err)
}

func dialUnix(t *testing.T, addr utils.NetAddr) net.Conn {
	sockPath := addr.Addr

	// Wait for the socket to be created.
	require.Eventually(t, func() bool {
		_, err := os.Stat(sockPath)
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		require.NoError(t, err)
		return true
	}, time.Millisecond*500, time.Millisecond*50)

	conn, err := net.DialTimeout("unix", sockPath, timeout)
	require.NoError(t, err)
	return conn
}

func dialTCP(t *testing.T, addr utils.NetAddr, certsDir string, createClientTLSConfigFunc createClientTLSConfigFunc) net.Conn {
	dialer := tls.Dialer{
		Config: createClientTLSConfigFunc(t, certsDir),
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(func() { cancel() })

	conn, err := dialer.DialContext(ctx, addr.AddrNetwork, addr.Addr)
	require.NoError(t, err)
	return conn
}

func createValidClientTLSConfig(t *testing.T, certsDir string) *tls.Config {
	// Hardcoded filenames under which Connect expects certs. In this test suite, we're trying to
	// reach the tsh gRPC server, so we need to use the renderer cert as the client cert.
	// The main process cert is created as well as the gRPC server is configured to expect both.
	clientCertPath := filepath.Join(certsDir, rendererCertFileName)
	mainProcessCertPath := filepath.Join(certsDir, mainProcessCertFileName)
	serverCertPath := filepath.Join(certsDir, tshdCertFileName)

	clientCert, err := generateAndSaveCert(clientCertPath, x509.ExtKeyUsageClientAuth)
	require.NoError(t, err)
	_, err = generateAndSaveCert(mainProcessCertPath, x509.ExtKeyUsageClientAuth)
	require.NoError(t, err)

	tlsConfig, err := createClientTLSConfig(clientCert, serverCertPath)
	require.NoError(t, err)

	// this would be done by the grpc TransportCredential in the grpc client,
	// but we're going to fake it with just a tls.Client, so we have to add the
	// http2 next proto ourselves (enforced by grpc-go starting from v1.67, and
	// required by the http2 spec when speaking http2 in TLS)
	if !slices.Contains(tlsConfig.NextProtos, "h2") {
		tlsConfig.NextProtos = append(tlsConfig.NextProtos, "h2")
	}

	return tlsConfig
}
