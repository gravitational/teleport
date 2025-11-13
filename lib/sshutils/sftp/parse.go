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

package sftp

import (
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils"
)

// Target is an SFTP destination to copy to or from.
type Target struct {
	// Login is a login username.
	Login string
	// Addr is the host and port to copy to/from. If nil, target is a local path.
	Addr *utils.NetAddr
	// Path is a path to copy to/from.
	// An empty path name is valid, and it refers to the user's default directory (usually
	// the user's home directory).
	// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
	Path string
}

// AddrString gets the target address, or the empty string if not set.
func (t Target) AddrString() string {
	if t.Addr != nil {
		return t.Addr.String()
	}
	return ""
}

var windowsDrivePattern = regexp.MustCompile(`^[A-Z]:\\`)

// IsRemotePath checks if a path refers to a location on a remote host.
func IsRemotePath(input string) bool {
	// Windows paths are always local; check here so we don't confuse the drive
	// name for a host.
	if windowsDrivePattern.MatchString(input) {
		return false
	}
	colonIndex := strings.Index(input, ":")
	if colonIndex == -1 {
		// Can't be remote without a colon.
		return false
	}
	slashIndex := strings.Index(input, string(os.PathSeparator))
	// On Unix, colons are valid in path names, so check if the first colon is
	// part of the path.
	//
	// If we get something like "foo:bar", we have no way to tell if that's local
	// path "foo:bar" or remote path "bar" on host "foo". We'll assume it's the
	// host:path variant, as the other can also be written as "./foo:bar".
	return slashIndex == -1 || colonIndex < slashIndex
}

// ParseTarget takes a string representing a resource for SFTP
// to download/upload in the form "[[user@]host:]path" and parses it into
// a structured form.
//
// Examples of valid targets:
//   - "/abs/or/rel/path"
//   - "host:/abs/or/rel/path"
//   - "user@host:/abs/or/rel/path"
//
// See https://tools.ietf.org/html/draft-ietf-secsh-filexfer-09#page-14, 'File Names'
// section about details on file names.
func ParseTarget(input string, port int) (Target, error) {
	if !IsRemotePath(input) {
		if !strings.Contains(input, "@") {
			return Target{
				Path: input,
			}, nil
		}
		// user@host, this is missing a path.
		return Target{}, trace.BadParameter("%q is missing a path, use form [[user@]host:]path", input)
	}
	firstColonIdx := strings.Index(input, ":")
	hostStartIdx := strings.LastIndex(input[:firstColonIdx], "@")
	// if a login exists and the path begins right after the login ends,
	// no host is specified
	if hostStartIdx != -1 && hostStartIdx+1 == firstColonIdx {
		return Target{}, trace.BadParameter("%q is missing a host, use form [[user@]host:]path", input)
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
			return Target{}, trace.Wrap(err)
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
			return Target{}, trace.Wrap(err)
		}
	}
	// Add port. Trim brackets first for ipv6 addresses as JoinHostPort will re-add them.
	host.Addr = net.JoinHostPort(strings.Trim(host.Host(), "[]"), strconv.Itoa(port))

	// if there is nothing after the host the path defaults to "."
	path := "."
	if len(input) > pathStartIdx {
		path = input[pathStartIdx:]
	}

	return Target{
		Login: login,
		Addr:  host,
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
		return nil, 0, trace.BadParameter("%q is missing a path, use form [[user@]host:]path", input)
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

// Sources is a list of paths to copy from either a local or remote host.
type Sources struct {
	// Login is a login username.
	Login string
	// Addr is the host and port to copy to/from. If nil, target is a local path.
	Addr *utils.NetAddr
	// Paths are the paths to copy from.
	Paths []string
}

// AddrString gets the target address, or the empty string if not set.
func (s Sources) AddrString() string {
	if s.Addr != nil {
		return s.Addr.String()
	}
	return ""
}

// ParseSources parses a list of strings representing resources for SFTP to
// download into a structured form (same format as [ParseTarget]).
func ParseSources(rawSources []string, port int) (Sources, error) {
	if len(rawSources) == 0 {
		return Sources{}, trace.BadParameter("at least one source required")
	}
	firstSource, err := ParseTarget(rawSources[0], port)
	if err != nil {
		return Sources{}, trace.Wrap(err)
	}
	sources := Sources{
		Login: firstSource.Login,
		Addr:  firstSource.Addr,
		Paths: []string{firstSource.Path},
	}
	for _, rawSource := range rawSources[1:] {
		source, err := ParseTarget(rawSource, port)
		if err != nil {
			return Sources{}, trace.Wrap(err)
		}
		if source.Login != sources.Login || source.AddrString() != sources.AddrString() {
			return Sources{}, trace.BadParameter("sources must all have the same user and host")
		}
		sources.Paths = append(sources.Paths, source.Path)
	}
	return sources, nil
}
