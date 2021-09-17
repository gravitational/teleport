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

package main

import (
	"context"
	"flag"
	"os"
	"syscall"

	"github.com/gravitational/teleport/lib/terminal"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

var (
	logFormat = flag.String("log_format", "", "Log format to use (json or text)")
	logLevel  = flag.String("log_level", "", "Log level to use")

	addr      = flag.String("addr", "tcp://localhost:", "Bind address for the Terminal server")
	certFile  = flag.String("cert_file", "", "Cert file (or inline PEM) for the Terminal server. Enables TLS.")
	certKey   = flag.String("cert_key", "", "Key file (or inline PEM) for the Terminal server. Enables TLS.")
	clientCAs = flag.String("client_cas", "", "Client CA certificate (or inline PEM) for the Terminal server. Enables mTLS.")
	stdin     = flag.Bool("stdin", false, "Read server configuration from stdin")
)

func main() {
	flag.Parse()
	configureLogging()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func configureLogging() {
	switch *logFormat {
	case "": // OK, use defaults
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		log.SetFormatter(&log.TextFormatter{})
	default:
		log.Warnf("Invalid log_format flag: %q", *logFormat)
	}
	if ll := *logLevel; ll != "" {
		switch level, err := log.ParseLevel(ll); {
		case err == nil:
			log.WithError(err).Warn("Invalid -log_level flag")
		default:
			log.SetLevel(level)
		}
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var trustedCAs []string
	if *clientCAs != "" {
		trustedCAs = []string{*clientCAs}
	}
	server, err := terminal.Start(ctx, terminal.ServerOpts{
		Addr:            *addr,
		CertFile:        *certFile,
		KeyFile:         *certKey,
		ClientCAs:       trustedCAs,
		ReadFromInput:   *stdin,
		ConfigInput:     os.Stdin,
		ConfigOutput:    os.Stdout,
		ShutdownSignals: []os.Signal{os.Interrupt, syscall.SIGTERM},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("tshd running at %v", server.Addr)
	return <-server.C
}
