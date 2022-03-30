/*
Copyright 2022 Gravitational, Inc.

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

package alpnproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardProxy(t *testing.T) {
	receiverCode := http.StatusAccepted
	receiverWant := func(req *http.Request) bool {
		return req.Host == "receiver.wanted.com:443"
	}
	receiver := mustCreateHTTPSListenerReceiver(t, receiverWant)
	go http.Serve(receiver, httpHandlerReturnsCode(receiverCode))

	originalHostCode := http.StatusCreated
	originalHostServer := httptest.NewTLSServer(httpHandlerReturnsCode(originalHostCode))

	t.Run("wanted and sent to receiver", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, receiver)
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		mustSuccessfullyCallHTTPSServerWithCode(
			t,
			"receiver.wanted.com:443",
			*client,
			receiverCode,
		)
	})

	t.Run("not wanted and sent to original host", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, receiver)
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		mustSuccessfullyCallHTTPSServerWithCode(
			t,
			originalHostServer.Listener.Addr().String(),
			*client,
			originalHostCode,
		)
	})

	t.Run("not wanted and dropped", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, receiver, func(config *ForwardProxyConfig) {
			config.DropUnwantedRequests = true
		})
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		// The "Bad Request" is returend to the CONNECT tunnel request. The
		// actual call never happens. Thus getting an error instead of a 4xx
		// status code.
		resp, err := client.Get(fmt.Sprintf("https://%s", originalHostServer.Listener.Addr().String()))
		require.Error(t, err)
		require.Contains(t, err.Error(), "Bad Request")
		require.NoError(t, resp.Body.Close())
	})

	t.Run("not wanted and system proxied", func(t *testing.T) {
		// An 2nd ForwardProxy is used here to simulate a system proxy:
		// client -> forward proxy -> system proxy -> system proxy receiver
		systemProxyReceiverCode := http.StatusNoContent
		systemProxyReceiver := mustCreateHTTPSListenerReceiver(t, nil)
		go http.Serve(systemProxyReceiver, httpHandlerReturnsCode(systemProxyReceiverCode))
		systemProxyHTTPServer := createForwardProxy(t, systemProxyReceiver)
		systemProxyHTTPAddr := "http://" + systemProxyHTTPServer.cfg.Listener.Addr().String()

		forwardProxy := createForwardProxy(t, receiver, withSystemProxyTo(systemProxyHTTPAddr))
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		mustSuccessfullyCallHTTPSServerWithCode(
			t,
			originalHostServer.Listener.Addr().String(),
			*client,
			systemProxyReceiverCode,
		)
	})

	t.Run("not wanted and system proxied (https)", func(t *testing.T) {
		// This test is the same as previous one except the system proxy is a
		// HTTPS server.
		systemProxyReceiverCode := http.StatusResetContent
		systemProxyReceiver := mustCreateHTTPSListenerReceiver(t, nil)
		go http.Serve(systemProxyReceiver, httpHandlerReturnsCode(systemProxyReceiverCode))
		systemProxyHTTPSServer := createForwardProxy(t, systemProxyReceiver,
			func(config *ForwardProxyConfig) {
				config.Listener = mustCreateLocalTLSListener(t)
			},
		)
		systemProxyHTTPSAddr := "https://" + systemProxyHTTPSServer.cfg.Listener.Addr().String()

		forwardProxy := createForwardProxy(t, receiver, withSystemProxyTo(systemProxyHTTPSAddr))
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		mustSuccessfullyCallHTTPSServerWithCode(
			t,
			originalHostServer.Listener.Addr().String(),
			*client,
			systemProxyReceiverCode,
		)
	})
}

func createForwardProxy(t *testing.T, receiver ForwardProxyReceiver, opts ...func(*ForwardProxyConfig)) *ForwardProxy {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	config := ForwardProxyConfig{
		Listener:            mustCreateLocalListener(t),
		CloseContext:        ctx,
		InsecureSystemProxy: true,
		Receivers:           []ForwardProxyReceiver{receiver},
	}

	for _, opt := range opts {
		opt(&config)
	}

	forwardProxy, err := NewForwardProxy(config)
	require.NoError(t, err)

	t.Cleanup(func() {
		forwardProxy.Close()
	})

	go func() {
		require.NoError(t, forwardProxy.Start())
	}()
	return forwardProxy
}

func withSystemProxyTo(proxyURL string) func(*ForwardProxyConfig) {
	// Default httpproxy.Config.ProxyFunc() bypasses proxy servers that are
	// localhost. Force it here for local testing.
	return func(config *ForwardProxyConfig) {
		config.SystemProxyFunc = func(*url.URL) (*url.URL, error) {
			return url.Parse(proxyURL)
		}
	}
}
