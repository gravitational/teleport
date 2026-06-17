/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"os"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/har"
)

// writeHAR reads HTTP recording events from the provided channels and writes a HAR 1.2 file to outputPath.
// The caller is responsible for creating the stream; the AppSessionStart event that identified this as
// an app session has already been consumed before this is called.
func writeHAR(ctx context.Context, evts <-chan apievents.AuditEvent, errs <-chan error, outputPath string, write func(format string, args ...any) (int, error)) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b := har.NewBuilder()
	entries := 0
loop:
	for {
		select {
		case err := <-errs:
			return trace.Wrap(err)
		case <-ctx.Done():
			return ctx.Err()
		case evt, more := <-evts:
			if !more {
				break loop
			}
			if _, ok := evt.(*apievents.AppSessionHTTPRequest); ok {
				entries++
			}
			b.Add(evt)
		}
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := b.Encode(f); err != nil {
		f.Close()
		return trace.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return trace.Wrap(err)
	}
	if write != nil {
		write("wrote %v (%d entries)\n", outputPath, entries)
	}
	return nil
}
