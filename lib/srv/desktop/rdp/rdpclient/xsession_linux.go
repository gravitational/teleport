/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package rdpclient

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func xSessionCommand(display string, userName string) []string {
	command := "startxfce4"
	if file, err := os.Open("/usr/share/xsessions/teleport.desktop"); err == nil {
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Exec=") {
				command = strings.TrimPrefix(line, "Exec=")
			}
		}
	}
	return []string{"su", "-c", fmt.Sprintf("env DISPLAY=%s %s", display, command), "-", userName}
}
