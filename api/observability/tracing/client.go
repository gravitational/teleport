// Copyright 2022 Gravitational, Inc
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

package tracing

import (
	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"google.golang.org/grpc"
)

// Client is a wrapper around an otlptrace.Client that provides a mechanism to
// close the underlying grpc.ClientConn. When an otlpgrpc.Client is constructed with
// the WithGRPCConn option, it is up to the caller to close the provided grpc.ClientConn.
// As such, we wrap and implement io.Closer to allow users to have a way to close the connection.
type Client struct {
	otlptrace.Client
	conn *grpc.ClientConn
}

// NewClient returns a new Client that uses the provided grpc.ClientConn to
// connect to the OpenTelemetry exporter.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		Client: otlptracegrpc.NewClient(otlptracegrpc.WithGRPCConn(conn)),
		conn:   conn,
	}
}

// Close closes the underlying grpc.ClientConn. This is required since when
// using otlptracegrpc.WithGRPCConn the otlptrace.Client does not
// close the connection when Shutdown is called.
func (c *Client) Close() error {
	return trace.Wrap(c.conn.Close())
}
