/**
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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// pollUntil calls probe at the given interval until it returns true,
// the timeout expires, or the context is cancelled.
func pollUntil(ctx context.Context, timeout, interval time.Duration, probe func(ctx context.Context) (bool, error)) error {
	deadline := time.After(timeout)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-deadline:
			return fmt.Errorf("timed out after %s", timeout)

		case <-tick.C:
			ok, err := probe(ctx)
			if err != nil {
				return err
			}

			if ok {
				return nil
			}
		}
	}
}

func resolveE2EDir() (string, error) {
	if v := os.Getenv("E2E_DIR"); v != "" {
		return filepath.Abs(v)
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	e2eDir := filepath.Dir(filepath.Dir(exePath))
	return e2eDir, nil
}
