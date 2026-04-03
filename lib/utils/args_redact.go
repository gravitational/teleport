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

package utils

import (
	"slices"
	"strings"
)

// ArgValueRedactor transforms a sensitive CLI flag value into a redacted form suitable for logging.
type ArgValueRedactor func(value string) string

// RedactFlagArgs returns a copy of args with sensitive flag values replaced by the output
// of the corresponding redactor function. This is used when logging command-line arguments
// that may contain secrets (e.g. join tokens passed to "teleport node configure" during
// EC2 auto-discovery), so that structured log output does not leak credentials.
//
// Supported formats:
//   - --flag=value
//   - --flag value
func RedactFlagArgs(args []string, redactors map[string]ArgValueRedactor) []string {
	redactedArgs := slices.Clone(args)
	for i := 0; i < len(redactedArgs); i++ {
		arg := redactedArgs[i]

		flag, value, hasInlineValue := strings.Cut(arg, "=")
		if redactor, ok := redactors[flag]; ok && hasInlineValue {
			redactedArgs[i] = flag + "=" + redactor(value)
			continue
		}

		if redactor, ok := redactors[arg]; ok && i+1 < len(redactedArgs) {
			redactedArgs[i+1] = redactor(redactedArgs[i+1])
			i++
		}
	}

	return redactedArgs
}
