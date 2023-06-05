/*
Copyright 2023 Gravitational, Inc.

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
	"net"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

func main() {
	cmd := &cobra.Command{
		Short: "A simple forward proxy that can be used as `HTTPS_PROXY` for testing.",
		Run: func(cmd *cobra.Command, args []string) {
			run(cmd, args)
		},
	}
	cmd.Flags().BoolP("debug", "d", true, "Enables debug logging")
	cmd.Flags().StringP("listen", "l", "0.0.0.0:80", "Listen addr")
	cmd.Flags().StringArrayP("route", "r", nil, "Route requests with host to a specific addr. Example: example.com:443=localhost:11443")
	if err := cmd.Execute(); err != nil {
		logrus.Fatal(err)
	}
}

func makeListener(cmd *cobra.Command) net.Listener {
	listenAddr, _ := cmd.Flags().GetString("listen")
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Infof("Listening on HTTPS_PROXY=http://%v", listenAddr)
	return listener
}

func makeHandlers(cmd *cobra.Command) (handlers []alpnproxy.ConnectRequestHandler) {
	// Custom routes.
	routes, _ := cmd.Flags().GetStringArray("route")
	for _, route := range routes {
		from, to, ok := strings.Cut(route, "=")
		if !ok {
			logrus.Warnf("Invalid --route %v", route)
			continue
		}

		handlers = append(handlers, alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
			MatchFunc: func(req *http.Request) bool {
				if req.Host == from {
					logrus.Debugf("Request %v will be reouted to %v", req.Host, to)
					return true
				}
				return false
			},
			Host: to,
		}))
	}

	// Catch all.
	handlers = append(handlers, alpnproxy.NewForwardToOriginalHostHandler())
	return
}

func run(cmd *cobra.Command, args []string) {
	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	forward, err := alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     makeListener(cmd),
		CloseContext: context.Background(),
		Handlers:     makeHandlers(cmd),
	})
	if err != nil {
		logrus.Fatal(err)
	}
	forward.Start()
}
