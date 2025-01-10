/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package lib

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/gravitational/trace"
)

// Bail exits with nonzero exit code and prints an error to a log.
func Bail(err error) {
	ctx := context.Background()
	var agg trace.Aggregate
	if errors.As(trace.Unwrap(err), &agg) {
		for i, err := range agg.Errors() {
			slog.ErrorContext(ctx, "Terminating with fatal error", "error_number", i+1, "error", err)
		}
	} else {
		slog.ErrorContext(ctx, "Terminating with fatal error", "error", err)
	}
	os.Exit(1)
}
