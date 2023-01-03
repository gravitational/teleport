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

package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewInsecureWebClientHTTPProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "fakeproxy.example.com:9999")
	client := NewInsecureWebClient()
	//nolint:bodyclose // resp should be nil, so there will be no body to close.
	resp, err := client.Get("https://fakedomain.example.com")
	// Client should try to proxy through nonexistent server at localhost.
	require.Error(t, err, "GET unexpectedly succeeded: %+v", resp)
	require.Contains(t, err.Error(), "proxyconnect")
	require.Contains(t, err.Error(), "lookup fakeproxy.example.com")
	require.Contains(t, err.Error(), "no such host")
}

func TestNewInsecureWebClientNoProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "fakeproxy.example.com:9999")
	t.Setenv("NO_PROXY", "fakedomain.example.com")
	client := NewInsecureWebClient()
	//nolint:bodyclose // resp should be nil, so there will be no body to close.
	resp, err := client.Get("https://fakedomain.example.com")
	require.Error(t, err, "GET unexpectedly succeeded: %+v", resp)
	require.NotContains(t, err.Error(), "proxyconnect")
	require.Contains(t, err.Error(), "lookup fakedomain.example.com")
	require.Contains(t, err.Error(), "no such host")
}

func TestNewClientWithPoolHTTPProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "fakeproxy.example.com:9999")
	client := newClientWithPool(nil)
	//nolint:bodyclose // resp should be nil, so there will be no body to close.
	resp, err := client.Get("https://fakedomain.example.com")
	// Client should try to proxy through nonexistent server at localhost.
	require.Error(t, err, "GET unexpectedly succeeded: %+v", resp)
	require.Contains(t, err.Error(), "proxyconnect")
	require.Contains(t, err.Error(), "lookup fakeproxy.example.com")
	require.Contains(t, err.Error(), "no such host")
}

func TestNewClientWithPoolNoProxy(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "fakeproxy.example.com:9999")
	t.Setenv("NO_PROXY", "fakedomain.example.com")
	client := newClientWithPool(nil)
	//nolint:bodyclose // resp should be nil, so there will be no body to close.
	resp, err := client.Get("https://fakedomain.example.com")
	require.Error(t, err, "GET unexpectedly succeeded: %+v", resp)
	require.NotContains(t, err.Error(), "proxyconnect")
	require.Contains(t, err.Error(), "lookup fakedomain.example.com")
	require.Contains(t, err.Error(), "no such host")
}
