/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tools

import (
	"fmt"
	"strings"
)

type progressWriter struct {
	n     int64
	limit int64
}

func (w *progressWriter) Write(p []byte) (int, error) {
	w.n = w.n + int64(len(p))

	n := int((w.n*100)/w.limit) / 10
	bricks := strings.Repeat("▒", n) + strings.Repeat(" ", 10-n)
	fmt.Print("\rUpdate progress: [" + bricks + "] (Ctrl-C to cancel update)")

	if w.n == w.limit {
		fmt.Print("\n")
	}

	return len(p), nil
}
