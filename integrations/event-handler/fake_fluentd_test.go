/*
Copyright 2015-2021 Gravitational, Inc.

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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integrations/lib/logger"
)

type FakeFluentd struct {
	keyTmpDir      string
	caCertPath     string
	caKeyPath      string
	clientCertPath string
	clientKeyPath  string
	serverCertPath string
	serverKeyPath  string

	server     *httptest.Server
	chMessages chan string
}

const (
	concurrency = 255
)

// NewFakeFluentd creates new unstarted fake server instance
func NewFakeFluentd(t *testing.T) *FakeFluentd {
	dir := t.TempDir()

	f := &FakeFluentd{keyTmpDir: dir}
	require.NoError(t, f.writeCerts())

	require.NoError(t, f.createServer())

	f.chMessages = make(chan string, concurrency)

	return f
}

// writeCerts generates and writes temporary mTLS keys
func (f *FakeFluentd) writeCerts() error {
	g, err := GenerateMTLSCerts([]string{"localhost"}, []string{}, time.Hour, 1024)
	if err != nil {
		return trace.Wrap(err)
	}

	f.caCertPath = path.Join(f.keyTmpDir, "ca.crt")
	f.caKeyPath = path.Join(f.keyTmpDir, "ca.key")
	f.serverCertPath = path.Join(f.keyTmpDir, "server.crt")
	f.serverKeyPath = path.Join(f.keyTmpDir, "server.key")
	f.clientCertPath = path.Join(f.keyTmpDir, "client.crt")
	f.clientKeyPath = path.Join(f.keyTmpDir, "client.key")

	err = g.CACert.WriteFile(f.caCertPath, f.caKeyPath, "")
	if err != nil {
		return trace.Wrap(err)
	}

	err = g.ServerCert.WriteFile(f.serverCertPath, f.serverKeyPath, "")
	if err != nil {
		return trace.Wrap(err)
	}

	err = g.ClientCert.WriteFile(f.clientCertPath, f.clientKeyPath, "")
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// createServer initializes new server instance
func (f *FakeFluentd) createServer() error {
	caCert, err := os.ReadFile(f.caCertPath)
	if err != nil {
		return trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(f.serverCertPath, f.serverKeyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig := &tls.Config{RootCAs: pool, Certificates: []tls.Certificate{cert}}

	f.server = httptest.NewUnstartedServer(http.HandlerFunc(f.Respond))
	f.server.TLS = tlsConfig

	return nil
}

// GetClientConfig returns FlientdConfig to connect to this fake fluentd server instance
func (f *FakeFluentd) GetClientConfig() FluentdConfig {
	return FluentdConfig{
		FluentdCA:   f.caCertPath,
		FluentdCert: f.clientCertPath,
		FluentdKey:  f.clientKeyPath,
	}
}

// Start starts fake fluentd server
func (f *FakeFluentd) Start() {
	f.server.StartTLS()
}

// Close closes fake fluentd server
func (f *FakeFluentd) Close() {
	f.server.Close()
	close(f.chMessages)
}

// GetURL returns fake server URL
func (f *FakeFluentd) GetURL() string {
	return f.server.URL
}

// Respond is the response function
func (f *FakeFluentd) Respond(w http.ResponseWriter, r *http.Request) {
	req, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Standard().WithError(err).Error("FakeFluentd Respond() failed to read body")
		fmt.Fprintln(w, "NOK")
		return
	}

	f.chMessages <- string(req)
	fmt.Fprintln(w, "OK")
}

// GetMessage reads next message from a mock server buffer
func (f *FakeFluentd) GetMessage(ctx context.Context) (string, error) {
	select {
	case message := <-f.chMessages:
		return message, nil
	case <-ctx.Done():
		return "", trace.Wrap(ctx.Err())
	}
}
