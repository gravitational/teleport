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
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Destination is a remote SFTP destination to copy to or from.
type Destination struct {
	// Login is an optional login username
	Login string
	// Host is a host to copy to/from
	Host *utils.NetAddr
	// Path is a path to copy to/from.
	// An empty path name is valid, and it refers to the user's default directory (usually
	// the user's home directory).
	// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
	Path string
}

// ParseDestination takes a string representing a remote resource for SFTP
// to download/upload in the form "[user@]host:[path]" and parses it into
// a structured form.
//
// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
// section about details on file names.
func ParseDestination(input string) (*Destination, error) {
	firstColonIdx := strings.Index(input, ":")
	// if there are no colons, no path is specified
	if firstColonIdx == -1 {
		return nil, trace.BadParameter("%q is missing a path, use form [user@]host:[path]", input)
	}
	hostStartIdx := strings.LastIndex(input[:firstColonIdx], "@")
	// if a login exists and the path begins right after the login ends,
	// no host is specified
	if hostStartIdx != -1 && hostStartIdx+1 == firstColonIdx {
		return nil, trace.BadParameter("%q is missing a host, use form [user@]host:[path]", input)
	}

	var login string
	// If at least one '@' exists and is before the first ':', get the
	// login. Otherwise, either there are no '@' or all '@' are after
	// the first ':' (where the host or path starts), so no login was
	// specified.
	if hostStartIdx != -1 {
		login = input[:hostStartIdx]
		// increment so that we won't try to parse the host starting at '@'
		hostStartIdx++
	} else {
		hostStartIdx = 0
	}

	// the path will start after the first colon, unless the host is an
	// IPv6 address
	pathStartIdx := firstColonIdx + 1
	var host *utils.NetAddr
	// if the host begins with '[', it is most likely an IPv6 address,
	// so attempt to parse it as such
	afterLogin := input[hostStartIdx:]
	if afterLogin[0] == '[' {
		ipv6Host, hostEndIdx, err := parseIPv6Host(input, hostStartIdx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if ipv6Host != nil {
			host = ipv6Host
			pathStartIdx = hostEndIdx
		}
	}

	// the host could not be parsed as an IPv6 address, try parsing it raw
	if host == nil {
		var err error
		host, err = utils.ParseAddr(input[hostStartIdx:firstColonIdx])
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// if there is nothing after the host the path defaults to "."
	path := "."
	if len(input) > pathStartIdx {
		path = input[pathStartIdx:]
	}

	return &Destination{
		Login: login,
		Host:  host,
		Path:  path,
	}, nil
}

// parseIPv6Host returns the parsed host in input and the index of input
// where the host ends. parseIPv6Host assumes the host contained in
// input starts with '['.
func parseIPv6Host(input string, start int) (*utils.NetAddr, int, error) {
	hostStr := input[start:]
	// if there is only one ':' in the entire input, the host isn't
	// an IPv6 address
	if strings.Count(hostStr, ":") == 1 {
		return nil, 0, trace.BadParameter("%q has an invalid host, host cannot contain '[' unless it is an IPv6 address", input)
	}
	// if there's no closing ']', this isn't a valid IPv6 address
	rbraceIdx := strings.Index(hostStr, "]")
	if rbraceIdx == -1 {
		return nil, 0, trace.BadParameter("%q has an invalid host, host cannot contain '[' or ':' unless it is an IPv6 address", input)
	}
	// if there's nothing after ']' then the path is missing
	if len(hostStr) <= rbraceIdx+2 {
		return nil, 0, trace.BadParameter("%q is missing a path, use form [user@]host:[path]", input)
	}

	maybeAddr := hostStr[:rbraceIdx+1]
	host, err := utils.ParseAddr(maybeAddr)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}

	// the host ends after the login + the IPv6 address
	// (including the trailing ']') and a ':'
	return host, start + rbraceIdx + 1 + 1, nil
}
