//go:build unix

// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package resumption

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/trace"
)

func sockPath(dataDir string, token resumptionToken) string {
	hash := sha256.Sum256(token[:])
	// unix domain sockets are limited to 108 or 104 characters, so spending 64
	// for the full sha256 hash is a bit too much; truncating the hash to 128
	// bits still gives us more than enough headroom to just assume that we'll
	// have no collisions (a probability of one in a quintillion with 26 billion
	// concurrent connections)
	return filepath.Join(dataDir, "handover", fmt.Sprintf("%x.sock", hash[:16]))
}

func sockDir(dataDir string) string {
	return filepath.Join(dataDir, "handover")
}

var errNoDataDir error = &trace.NotFoundError{Message: "data dir not configured"}

func (r *SSHServerWrapper) listenHandover(token resumptionToken) (net.Listener, error) {
	if r.dataDir == "" {
		return nil, trace.Wrap(errNoDataDir)
	}

	_ = os.MkdirAll(sockDir(r.dataDir), 0o700)
	l, err := net.Listen("unix", sockPath(r.dataDir, token))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return l, nil
}

func (r *SSHServerWrapper) dialHandover(token resumptionToken) (net.Conn, error) {
	if r.dataDir == "" {
		return nil, trace.Wrap(errNoDataDir)
	}

	c, err := net.DialTimeout("unix", sockPath(r.dataDir, token), time.Second)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return c, nil
}

func filterNonConnectableSockets(ctx context.Context, paths []string) (filtered []string, lastErr error) {
	filtered = paths[:0]

	var d net.Dialer
	for _, path := range paths {
		c, err := d.DialContext(ctx, "unix", path)
		if err == nil {
			_ = c.Close()
			continue
		}

		if errors.Is(err, os.ErrNotExist) {
			continue
		}

		if errors.Is(err, syscall.ECONNREFUSED) {
			filtered = append(filtered, path)
			continue
		}

		lastErr = trace.ConvertSystemError(err)
	}

	return filtered, lastErr
}

func (r *SSHServerWrapper) HandoverCleanup(ctx context.Context) error {
	if r.dataDir == "" {
		return nil
	}

	dir := sockDir(r.dataDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return trace.ConvertSystemError(err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".sock") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}

	paths, firstErr := filterNonConnectableSockets(ctx, paths)

	if len(paths) < 1 {
		return trace.Wrap(firstErr)
	}

	// unix domain sockets exist on disk between bind() and listen() but
	// connecting before listen() results in ECONNREFUSED, so we just wait a
	// little bit before testing them again; the first check lets us be done
	// with the check immediately in the happy case where there's no
	// unconnectable sockets
	r.log.WithField("sockets", len(paths)).Debug("Found some non-connectable handover sockets, waiting before checking them again.")

	select {
	case <-ctx.Done():
		return trace.NewAggregate(firstErr, ctx.Err())
	case <-time.After(3 * time.Second):
	}

	paths, secondErr := filterNonConnectableSockets(ctx, paths)

	if len(paths) < 1 {
		r.log.Debug("Found no non-connectable handover socket after waiting.")
		return trace.NewAggregate(firstErr, secondErr)
	}

	r.log.WithField("sockets", len(paths)).Info("Cleaning up some non-connectable handover sockets, left over from previous Teleport instances.")

	errs := []error{firstErr, secondErr}
	for _, path := range paths {
		errs = append(errs, trace.ConvertSystemError(os.Remove(path)))
	}

	return trace.NewAggregate(errs...)
}
