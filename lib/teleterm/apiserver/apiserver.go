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

package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"

	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/apiserver/handler"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	log "github.com/sirupsen/logrus"
)

// New creates an instance of API Server
func New(cfg Config) (*APIServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	serviceHandler, err := handler.New(
		handler.Config{
			DaemonService: cfg.Daemon,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ls, err := newListener(cfg.HostAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grpcCredentials, err := getGrpcCredentials(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grpcServer := grpc.NewServer(grpcCredentials, grpc.ChainUnaryInterceptor(
		withErrorHandling(cfg.Log),
	))

	api.RegisterTerminalServiceServer(grpcServer, serviceHandler)

	return &APIServer{cfg, ls, grpcServer}, nil
}

// Serve starts accepting incoming connections
func (s *APIServer) Serve() error {
	return s.grpcServer.Serve(s.ls)
}

// Stop stops the server and closes all listeners
func (s *APIServer) Stop() {
	s.grpcServer.GracefulStop()
}

func newListener(hostAddr string) (net.Listener, error) {
	uri, err := utils.ParseAddr(hostAddr)

	if err != nil {
		return nil, trace.BadParameter("invalid host address: %s", hostAddr)
	}

	lis, err := net.Listen(uri.Network(), uri.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addr := utils.FromAddr(lis.Addr())
	sendBoundNetworkPortToStdout(addr)

	log.Infof("tsh daemon is listening on %v.", addr.FullAddress())

	return lis, nil
}

func sendBoundNetworkPortToStdout(addr utils.NetAddr) {
	fmt.Printf("{CONNECT_GRPC_PORT: %v}\n", addr.Port(1))
}

// Server is a combination of the underlying grpc.Server and its RuntimeOpts.
type APIServer struct {
	Config
	// ls is the server listener
	ls net.Listener
	// grpc is an instance of grpc server
	grpcServer *grpc.Server
}

func getGrpcCredentials(cfg Config) (grpc.ServerOption, error) {
	uri, err := utils.ParseAddr(cfg.HostAddr)

	if err != nil {
		return nil, trace.BadParameter("invalid host address: %s", cfg.HostAddr)
	}

	if uri.Network() != "unix" {
		keyPair, err := generateKeyPair(cfg.CertsDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return grpc.Creds(keyPair), nil
	}

	return grpc.Creds(nil), nil
}

func generateKeyPair(certsDir string) (credentials.TransportCredentials, error) {
	cert, err := utils.GenerateSelfSignedCert([]string{"localhost"})
	if err != nil {
		return nil, trace.Wrap(err, "failed to generate a certificate")
	}

	err = os.WriteFile(filepath.Join(certsDir, "tsh_server.crt"), cert.Cert, 0600)
	if err != nil {
		return nil, trace.Wrap(err, "failed to save server certificate")
	}

	certificate, err := tls.X509KeyPair(cert.Cert, cert.PrivateKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to load server certificates")
	}

	tlsConfig := &tls.Config{
		GetConfigForClient: func(info *tls.ClientHelloInfo) (*tls.Config, error) {
			caCert, err := os.ReadFile(filepath.Join(certsDir, "client.crt"))
			if err != nil {
				return nil, trace.Wrap(err, "failed to read client certificate file")
			}
			caPool := x509.NewCertPool()
			if !caPool.AppendCertsFromPEM(caCert) {
				return nil, trace.Wrap(err, "failed to add client CA file")
			}
			return &tls.Config{
				ClientAuth:   tls.RequireAndVerifyClientCert,
				Certificates: []tls.Certificate{certificate},
				ClientCAs:    caPool,
			}, nil
		},
	}
	return credentials.NewTLS(tlsConfig), nil
}
