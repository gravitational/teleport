/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sftp

import (
	"regexp"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

var reSFTP = regexp.MustCompile(
	// optional username, note that outside group
	// is a non-capturing as it includes @ signs we don't want
	`(?:(?P<username>.+)@)?` +
		// either some stuff in brackets - [ipv6]
		// or some stuff without brackets and colons
		`(?P<host>` +
		// this says: [stuff in brackets that is not brackets] - loose definition of the IP address
		`(?:\[[^@\[\]]+\])` +
		// or
		`|` +
		// some stuff without brackets or colons to make sure the OR condition
		// is not ambiguous
		`(?:[^@\[\:\]]+)` +
		`)` +
		// after colon, there is a path that could consist technically of
		// any char including empty which stands for the implicit home directory
		`:(?P<path>.*)`,
)

// Destination is SCP destination to copy to or from
type Destination struct {
	// Login is an optional login username
	Login string
	// Host is a host to copy to/from
	Host utils.NetAddr
	// Path is a path to copy to/from.
	// An empty path name is valid, and it refers to the user's default directory (usually
	// the user's home directory).
	// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
	Path string
}

// ParseSCPDestination takes a string representing a remote resource for SFTP
// to download/upload, like "user@host:/path/to/resource.txt" and parses it into
// a structured form.
//
// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
// section about details on file names.
func ParseDestination(s string) (*Destination, error) {
	out := reSFTP.FindStringSubmatch(s)
	if len(out) < 4 {
		return nil, trace.BadParameter("failed to parse %q, try form user@host:/path", s)
	}
	addr, err := utils.ParseAddr(out[2])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	path := out[3]
	if path == "" {
		path = "."
	}
	return &Destination{Login: out[1], Host: *addr, Path: path}, nil
}
