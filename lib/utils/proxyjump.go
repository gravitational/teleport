/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/utilsaddr"
)

var reProxyJump = regexp.MustCompile(
	// optional username, note that outside group
	`(?:(?P<username>[^\:]+)@)?(?P<hostport>[^\@]+)`,
)

// JumpHost is a target jump host
type JumpHost struct {
	// Username to login as
	Username string
	// Addr is a target addr
	Addr utilsaddr.NetAddr
}

// ParseProxyJump parses strings like user@host:port,bob@host:port
func ParseProxyJump(in string) ([]JumpHost, error) {
	if in == "" {
		return nil, trace.BadParameter("missing proxyjump")
	}
	parts := strings.Split(in, ",")
	out := make([]JumpHost, 0, len(parts))
	for _, part := range parts {
		match := reProxyJump.FindStringSubmatch(strings.TrimSpace(part))
		if len(match) == 0 {
			return nil, trace.BadParameter("could not parse %q, expected format user@host:port,user@host:port", in)
		}
		addr, err := utilsaddr.ParseAddr(match[2])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, JumpHost{Username: match[1], Addr: *addr})
	}
	return out, nil
}
