// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package ipc (Inter-Process Communication) provides an encrypted communication
// channel between independent processes running on the same host.
//
// For example: Teleport Connect implements the Hardware Key Agent service using
// this package, enabling multiple Teleport clients (e.g. instances of tsh) to
// share the same PIV connection, cache and reuse the PIN, etc.
//
// Processes using this package will communicate via gRPC over a unix domain
// socket. Traffic is TLS-encrypted using a self-signed server certificate to
// prevent eavesdropping, however no client certificate is required.
//
// Authentication is controlled using file permissions of the socket, server cert,
// and the containing directory. [NewServer] will create the directory with mode
// 0700, the certificate with mode 0600, and the socket with mode 0777 so it can
// only be accessed by the current user.
//
// The [Server] type implements the [grpc.ServiceRegistrar] interface so it can
// be passed directly to protobuf service registration functions. Similarly,
// [NewClient] returns a [grpc.ClientConn] so it can be passed directly to
// protobuf client constructors.
//
// Example Usage:
//
//	const pathToDir = "/path/to/dir"
//
//	func runServer(ctx context.Context) error {
//		server, err := ipc.NewServer(ctx, pathToDir)
//		if err != nil {
//			return err
//		}
//		healthpb.RegisterHealthServer(server, health.NewServer())
//		return server.Serve(ctx)
//	}
//
//	func runClient(ctx context.Context) error {
//		client, err := ipc.NewClient(pathToDir)
//		if err != nil {
//			return err
//		}
//		defer client.Close()
//
//		_, err := healthpb.NewHealthClient(client).
//			Check(ctx, &healthpb.HealthCheckRequest{})
//		return err
//	}
package ipc
