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

package terminal

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strings"

	"github.com/gravitational/teleport/lib/terminal/terminalv1"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	terminalpb "github.com/gravitational/teleport/protogen/teleport/terminal/v1"
	log "github.com/sirupsen/logrus"
)

// ServerOpts contains configuration options for a Teleport Terminal service.
// Note that some options may be passed in JSON format via ConfigInput as long
// as ReadFromInput is set to true. If that is the case, then ConfigInput takes
// precedence over previously-configured values, even if the values set by it
// are empty.
type ServerOpts struct {
	// Addr is the bind address for the server, either in "scheme://host:port"
	// format (allowing for "tcp", "unix", etc) or in "host:port" format (defaults
	// to "tcp").
	Addr string `json:"addr"`

	// CertFile, KeyFile and ClientCAs are either file paths or the raw PEM
	// contents for the corresponding certificates or keys.
	// If CertFile and KeyFile are present, TLS is enabled.
	// Additionally, if ClientCAs is present, then mTLS is enabled.
	CertFile  string   `json:"cert_file"`
	KeyFile   string   `json:"key_file"`
	ClientCAs []string `json:"client_cas"`

	// ReadFromInput and ConfigInput configure the (optional) JSON override for
	// ServerOpts.
	ReadFromInput bool      `json:"-"`
	ConfigInput   io.Reader `json:"-"`

	// ConfigOutput, if present, is the destination for the JSON-marshaled
	// RuntimeOpts that is sent before the server is started.
	ConfigOutput io.Writer `json:"-"`

	// DisableReflect disables gRPC reflection.
	DisableReflect bool `json:"disable_reflect"`

	// ShutdownSignals is the set of captured signals that cause server shutdown.
	ShutdownSignals []os.Signal `json:"-"`
}

// RuntimeOpts contains the set of ServerOpts that may be dynamically assigned
// during server start (eg, a chosen random port for the server bind address).
type RuntimeOpts struct {
	// Addr is the server bind address (eg: "tcp://localhost:1234").
	Addr string `json:"addr"`
	// DialAddr is a Dial-friendly address.
	DialAddr string `json:"-"`
	TLS      bool   `json:"tls,omitempty"`
	MTLS     bool   `json:"mtls,omitempty"`
}

// Server is a combination of the underlying grpc.Server and its RuntimeOpts.
type Server struct {
	// C is the exit channel for a running server.
	// To stop the server close the context passed to Start.
	C <-chan error `json:"-"`
	*RuntimeOpts
}

// Start creates and starts a Teleport Terminal service.
// If ReadFromInput is present, then server a JSON-encoded version of opts is
// read from ConfigInput, overriding any settings present.
// If ConfigOutput is present, then a JSON-marshaled RuntimeOpts is sent to it
// before server start.
// Closing ctx stops the server.
func Start(ctx context.Context, opts ServerOpts) (*Server, error) {
	if opts.ReadFromInput {
		if err := json.NewDecoder(opts.ConfigInput).Decode(&opts); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	tlsOpts, err := parseTLSOpts(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var network, addr string
	switch tmp := strings.Split(opts.Addr, "://"); len(tmp) {
	case 1: // addr without network
		network = "tcp"
		addr = opts.Addr
	case 2: // addr with network
		network = tmp[0]
		addr = tmp[1]
	default:
		return nil, trace.BadParameter("unexpected addr format: %v", opts.Addr)
	}
	lis, err := net.Listen(network, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// `server` takes ownership of `lis` in the happy path, but let's make sure we
	// don't leak it in case of errors.
	ok := false
	defer func() {
		if ok {
			return
		}
		lis.Close()
	}()

	// Prepare addresses for RuntimeOpts.
	fullAddr := fmt.Sprintf("%v://%v", network, lis.Addr().String())
	dialAddr := fullAddr
	if network == "tcp" {
		dialAddr = lis.Addr().String()
	}

	// Construct RuntimeOpts and marshal to stdout.
	runtimeOpts := &RuntimeOpts{
		Addr:     fullAddr,
		DialAddr: dialAddr,
		TLS:      tlsOpts.TLS,
		MTLS:     tlsOpts.MTLS,
	}
	if out := opts.ConfigOutput; out != nil {
		if err := json.NewEncoder(out).Encode(runtimeOpts); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	server := grpc.NewServer(
		tlsOpts.Creds,
		// TODO(codingllama): Error translation, error logging interceptor.
	)
	terminalpb.RegisterTerminalServiceServer(server, &terminalv1.Service{})
	if !opts.DisableReflect {
		reflection.Register(server)
	}

	// Start service.
	serveC := make(chan error)
	go func() {
		err := server.Serve(lis)
		serveC <- err
		close(serveC)
	}()

	// Wait for shutdown signals
	go func() {
		c := make(chan os.Signal, len(opts.ShutdownSignals))
		signal.Notify(c, opts.ShutdownSignals...)
		select {
		case <-ctx.Done():
			log.Info("Context closed, stopping service")
		case sig := <-c:
			log.Infof("Captured %s, stopping service", sig)
		}
		server.GracefulStop()
	}()

	ok = true
	return &Server{
		C:           serveC,
		RuntimeOpts: runtimeOpts,
	}, nil
}

type tlsOpts struct {
	Creds     grpc.ServerOption
	TLS, MTLS bool
}

func parseTLSOpts(opts ServerOpts) (*tlsOpts, error) {
	// Should we enable TLS?
	var serverCert tls.Certificate
	switch {
	case opts.CertFile != "" && opts.KeyFile != "":
		var err error
		serverCert, err = parseKeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case opts.CertFile != "" || opts.KeyFile != "":
		return nil, trace.BadParameter("cert file and key file must be both present or both absent")
	}
	enableTLS := len(serverCert.Certificate) > 0

	// Should we enable mTLS?
	var clientCAs *x509.CertPool
	switch {
	case len(opts.ClientCAs) > 0 && enableTLS:
		var err error
		clientCAs, err = parseClientCAs(opts.ClientCAs)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case len(opts.ClientCAs) > 0:
		return nil, trace.BadParameter("client CAs provided without cert file and key file")
	}

	// Bail early if there is nothing to configure.
	if !enableTLS {
		return &tlsOpts{
			Creds: grpc.Creds(nil),
		}, nil
	}

	var clientAuth tls.ClientAuthType
	if clientCAs != nil {
		clientAuth = tls.RequireAndVerifyClientCert
	}
	return &tlsOpts{
		Creds: grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientAuth:   clientAuth,
			ClientCAs:    clientCAs,
			MinVersion:   tls.VersionTLS13,
		})),
		TLS:  true,
		MTLS: clientCAs != nil,
	}, nil
}

func parseKeyPair(certFile, keyFile string) (tls.Certificate, error) {
	// Try and parse as PEMs.
	pair, err := tls.X509KeyPair([]byte(certFile), []byte(keyFile))
	if err == nil {
		return pair, nil
	}
	log.WithError(err).Debug("Failed to parse key pair as PEMs, trying the filesystem")

	pair, err = tls.LoadX509KeyPair(certFile, keyFile)
	return pair, trace.Wrap(err)
}

func parseClientCAs(clientCAs []string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for i, cert := range clientCAs {
		// Try to parse as PEM.
		if pool.AppendCertsFromPEM([]byte(cert)) {
			continue
		}
		log.Debugf("Failed to parse client CA #%v as PEM, trying the filesystem", i+1)

		certBytes, err := ioutil.ReadFile(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !pool.AppendCertsFromPEM(certBytes) {
			return nil, trace.BadParameter("client CA #%v is not a valid certificate", i+1)
		}
	}
	return pool, nil
}
