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

package utils

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
)

// TryReadValueAsFile is a utility function to read a value
// from the disk if it looks like an absolute path,
// otherwise, treat it as a value.
// It only support absolute paths to avoid ambiguity in interpretation of the value
func TryReadValueAsFile(value string) (string, error) {
	if !filepath.IsAbs(value) {
		return value, nil
	}
	// treat it as an absolute filepath
	contents, err := os.ReadFile(value)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	// trim newlines as tokens in files tend to have newlines
	out := strings.TrimSpace(string(contents))

	if out == "" {
		slog.WarnContext(context.Background(), "Empty config value file", "file", value)
	}
	return out, nil
}
