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

package reexec

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/gravitational/trace"
)

// WaitForSignal waits for the other side of the pipe to signal. If not
// received, it will stop waiting and exit.
func WaitForSignal(ctx context.Context, r io.Reader, timeout time.Duration) error {
	waitCh := make(chan error, 1)
	go func() {
		// Reading from the file descriptor will block until it's closed.
		_, err := r.Read(make([]byte, 1))
		if errors.Is(err, io.EOF) {
			err = nil
		}
		waitCh <- err
	}()

	// Timeout if no signal has been sent within the provided duration.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return trace.Wrap(ctx.Err(), "got context error while waiting for signal")
	case <-timer.C:
		return trace.LimitExceeded("timed out waiting for signal")
	case err := <-waitCh:
		return trace.Wrap(err)
	}
}
