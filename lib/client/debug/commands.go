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

package debug

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
)

// Commander is the subset of Client used by the shared command functions.
// Both the local Unix-socket client and the tunnel-backed client satisfy it.
type Commander interface {
	GetLogLevel(ctx context.Context) (string, error)
	SetLogLevel(ctx context.Context, level string) (string, error)
	GetReadiness(ctx context.Context) (Readiness, error)
	GetRawMetrics(ctx context.Context) (io.ReadCloser, error)
}

// WriteLogLevel fetches the current log level and writes it to w.
func WriteLogLevel(ctx context.Context, w io.Writer, clt Commander) error {
	level, err := clt.GetLogLevel(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(w, strings.TrimSpace(level))
	return nil
}

// WriteSetLogLevel sets the log level and writes the result to w.
func WriteSetLogLevel(ctx context.Context, w io.Writer, clt Commander, level string) error {
	msg, err := clt.SetLogLevel(ctx, level)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Fprintln(w, strings.TrimSpace(msg))
	return nil
}

// WriteReadiness checks readiness and writes a human-readable status to w.
func WriteReadiness(ctx context.Context, w io.Writer, clt Commander) error {
	readiness, err := clt.GetReadiness(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if readiness.Ready {
		fmt.Fprintf(w, "Ready (PID %d, status: %s)\n", readiness.PID, readiness.Status)
	} else {
		fmt.Fprintf(w, "Not ready (PID %d, status: %s)\n", readiness.PID, readiness.Status)
	}
	return nil
}

// WriteMetrics fetches raw Prometheus metrics and copies them to w.
func WriteMetrics(ctx context.Context, w io.Writer, clt Commander) error {
	body, err := clt.GetRawMetrics(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer body.Close()
	_, err = io.Copy(w, body)
	return trace.Wrap(err)
}
