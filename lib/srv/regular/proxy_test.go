/*
Copyright 2016 Gravitational, Inc.

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

package regular

import (
	"bytes"
	"io"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/srv"
)

func TestParseProxyRequest(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc, req string
		expected  proxySubsysRequest
	}{
		{
			desc: "proxy request for a host:port",
			req:  "proxy:host:22",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "host",
				port:        "22",
				clusterName: "",
			},
		},
		{
			desc: "similar request, just with '@' at the end (missing site)",
			req:  "proxy:host:22@",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "host",
				port:        "22",
				clusterName: "",
			},
		},
		{
			desc: "proxy request for just the sitename",
			req:  "proxy:@moon",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "",
				port:        "",
				clusterName: "moon",
			},
		},
		{
			desc: "proxy request for the host:port@sitename",
			req:  "proxy:station:100@moon",
			expected: proxySubsysRequest{
				namespace:   "",
				host:        "station",
				port:        "100",
				clusterName: "moon",
			},
		},
		{
			desc: "proxy request for the host:port@namespace@cluster",
			req:  "proxy:station:100@system@moon",
			expected: proxySubsysRequest{
				namespace:   "system",
				host:        "station",
				port:        "100",
				clusterName: "moon",
			},
		},
	}

	for i, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.expected.namespace == "" {
				// test cases without a defined namespace are testing for
				// the presence of the default namespace; namespace should
				// never actually be empty.
				tt.expected.namespace = apidefaults.Namespace
			}
			req, err := parseProxySubsysRequest(tt.req)
			require.NoError(t, err, "Test case %d: req=%s, expected=%+v", i, tt.req, tt.expected)
			require.Equal(t, tt.expected, req, "Test case %d: req=%s, expected=%+v", i, tt.req, tt.expected)
		})
	}
}

func TestParseBadRequests(t *testing.T) {
	t.Parallel()

	server := &Server{
		hostname:  "redhorse",
		proxyMode: true,
	}

	ctx := &srv.ServerContext{}

	testCases := []struct {
		desc  string
		input string
	}{
		{desc: "empty request", input: "proxy:"},
		{desc: "missing hostname", input: "proxy::80"},
		{desc: "missing hostname and missing cluster name", input: "proxy:@"},
		{desc: "just random string", input: "this is bad string"},
	}
	for _, tt := range testCases {
		t.Run(tt.desc, func(t *testing.T) {
			subsystem, err := parseProxySubsys(tt.input, server, ctx)
			require.Error(t, err, "test case: %q", tt.input)
			require.Nil(t, subsystem, "test case: %q", tt.input)
		})
	}
}

func TestCheckedPrefixReader(t *testing.T) {
	getMockReadWriteCloser := func(data []byte) io.ReadWriteCloser {
		buf := bytes.NewBuffer(data)
		return struct {
			io.ReadWriter
			io.Closer
		}{
			ReadWriter: buf,
			Closer:     io.NopCloser(buf),
		}
	}

	testData := []byte("test data")
	t.Run("missing prefix", func(t *testing.T) {
		t.Run("single read", func(t *testing.T) {
			cr := checkedPrefixReader{
				ReadWriteCloser: getMockReadWriteCloser(testData),
				requiredPrefix:  []byte("wrong"),
			}

			_, err := io.ReadAll(&cr)
			require.True(t, trace.IsAccessDenied(err), "expected trace.AccessDenied error, got: %v", err)
		})
		t.Run("two reads", func(t *testing.T) {
			cr := checkedPrefixReader{
				ReadWriteCloser: getMockReadWriteCloser(testData),
				requiredPrefix:  []byte("tesD"),
			}

			buf := make([]byte, 3)
			n, err := cr.Read(buf)
			require.NoError(t, err)
			require.Equal(t, len(buf), n)
			require.Equal(t, testData[:len(buf)], buf)

			_, err = cr.Read(buf)
			require.True(t, trace.IsAccessDenied(err), "expected trace.AccessDenied error, got: %v", err)
		})
	})
	t.Run("success", func(t *testing.T) {
		t.Run("single read", func(t *testing.T) {
			cr := checkedPrefixReader{
				ReadWriteCloser: getMockReadWriteCloser(testData),
				requiredPrefix:  []byte("test"),
			}

			res, err := io.ReadAll(&cr)
			require.NoError(t, err)
			require.Equal(t, testData, res)

			secondData := []byte("second data")
			_, err = cr.Write(secondData)
			require.NoError(t, err)

			res, err = io.ReadAll(&cr)
			require.NoError(t, err)
			require.Equal(t, secondData, res)
		})
		t.Run("three reads", func(t *testing.T) {
			cr := checkedPrefixReader{
				ReadWriteCloser: getMockReadWriteCloser(testData),
				requiredPrefix:  []byte("test"),
			}

			buf := make([]byte, 3)
			n, err := cr.Read(buf)
			require.NoError(t, err)
			require.Equal(t, len(buf), n)
			require.Equal(t, testData[:len(buf)], buf)

			n, err = cr.Read(buf)
			require.NoError(t, err)
			require.Equal(t, len(buf), n)
			require.Equal(t, testData[len(buf):2*len(buf)], buf)

			n, err = cr.Read(buf)
			require.NoError(t, err)
			require.Equal(t, len(buf), n)
			require.Equal(t, testData[2*len(buf):], buf)

			secondData := []byte("second data")
			_, err = cr.Write(secondData)
			require.NoError(t, err)

			res, err := io.ReadAll(&cr)
			require.NoError(t, err)
			require.Equal(t, secondData, res)
		})

	})
}
