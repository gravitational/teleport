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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardProxy(t *testing.T) {
	receiverListener := mustCreateHTTPSListenerReceiverForAWS(t)
	receiverHandler := func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}
	go http.Serve(receiverListener, http.HandlerFunc(receiverHandler))

	notWantedServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	t.Run("wanted by receiver and send to receiver", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, false, receiverListener)
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		mustSuccessfullyCallHTTPSServerWithCode(
			t,
			"test.service.localhost.amazonaws.com",
			*client,
			http.StatusForbidden,
		)
	})

	t.Run("not wanted and send to host", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, false, receiverListener)
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		mustSuccessfullyCallHTTPSServerWithCode(
			t,
			notWantedServer.Listener.Addr().String(),
			*client,
			http.StatusNoContent,
		)
	})

	t.Run("not wanted and dropped", func(t *testing.T) {
		forwardProxy := createForwardProxy(t, true, receiverListener)
		client := httpsClientWithProxyURL(forwardProxy.cfg.Listener.Addr().String())

		_, err := client.Get(fmt.Sprintf("https://%s", notWantedServer.Listener.Addr().String()))
		require.Error(t, err)
		require.Contains(t, err.Error(), "Bad Request")
	})
}

func createForwardProxy(t *testing.T, dropUnwanted bool, receivers ...ForwardProxyReceiver) *ForwardProxy {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	config := ForwardProxyConfig{
		Listener:            mustCreateLocalListener(t),
		CloseContext:        ctx,
		DropUnwantedRequest: dropUnwanted,
		InsecureSystemProxy: true,
	}
	for _, receiver := range receivers {
		config.Receivers = append(config.Receivers, receiver)
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
