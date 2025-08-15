/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package internal

import (
	"context"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

func CreateListener(ctx context.Context, log *slog.Logger, addr string) (net.Listener, error) {
	parsed, err := url.Parse(addr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing %q", addr)
	}

	switch parsed.Scheme {
	// If no scheme is provided, default to TCP.
	case "tcp", "":
		return net.Listen("tcp", parsed.Host)
	case "unix":
		absPath, err := filepath.Abs(parsed.Path)
		if err != nil {
			return nil, trace.Wrap(err, "resolving absolute path for %q", parsed.Path)
		}

		// Remove the file if it already exists. This is necessary to handle
		// unclean exits.
		if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
			log.WarnContext(ctx, "Failed to remove existing socket file", "error", err)
		}

		l, err := net.ListenUnix("unix", &net.UnixAddr{
			Net:  "unix",
			Name: absPath,
		})
		if err != nil {
			return nil, trace.Wrap(err, "creating unix socket %q", absPath)
		}

		// On Unix systems, you must have read and write permissions for the
		// socket to connect to it. The execute permission on the directories
		// containing the socket must also be granted. This is different to when
		// we write output artifacts which only require the consumer to have
		// read access.
		//
		// We set the socket perm to 777. Instead of controlling access via
		// the socket file directly, users will either:
		// - Configure Unix Workload Attestation to restrict access to specific
		//   PID/UID/GID combinations.
		// - Configure the filesystem permissions of the directory containing
		//   the socket.
		if err := os.Chmod(absPath, os.ModePerm); err != nil {
			return nil, trace.Wrap(err, "setting permissions on unix socket %q", absPath)
		}

		return l, nil
	default:
		return nil, trace.BadParameter("unsupported scheme %q", parsed.Scheme)
	}
}
