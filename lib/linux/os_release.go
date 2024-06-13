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

package linux

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/gravitational/trace"
)

// OSRelease represents the information contained in the /etc/os-release file.
type OSRelease struct {
	PrettyName      string
	Name            string
	VersionID       string
	Version         string
	VersionCodename string
	ID              string
	IDLike          string
}

// ParseOSRelease reads the /etc/os-release contents.
func ParseOSRelease() (*OSRelease, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	return ParseOSReleaseFromReader(f)
}

// ParseOSReleaseFromReader reads an /etc/os-release data stream from in.
func ParseOSReleaseFromReader(in io.Reader) (*OSRelease, error) {
	m := make(map[string]string)

	scan := bufio.NewScanner(in)
	for scan.Scan() {
		line := scan.Text()

		line = strings.TrimSpace(line) // Remove spaces from start and end.

		if len(line) == 0 {
			continue // Skip empty lines.
		}

		if line[0] == '#' {
			continue // Skip comments.
		}

		vals := strings.Split(line, "=")
		if len(vals) != 2 {
			continue // Skip unexpected line.
		}

		key := strings.TrimSpace(vals[0])
		val := strings.Trim(vals[1], `"' `)
		m[key] = val
	}
	if err := scan.Err(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &OSRelease{
		PrettyName:      m["PRETTY_NAME"],
		Name:            m["NAME"],
		VersionID:       m["VERSION_ID"],
		Version:         m["VERSION"],
		VersionCodename: m["VERSION_CODENAME"],
		ID:              m["ID"],
		IDLike:          m["ID_LIKE"],
	}, nil
}
