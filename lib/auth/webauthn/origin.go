// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webauthn

import (
	"net/url"
	"strings"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func validateOrigin(origin, rpID string) error {
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return trace.BadParameter("origin is not a valid URL: %v", err)
	}
	host, err := utils.Host(parsedOrigin.Host)
	if err != nil {
		return trace.BadParameter("extracting host from origin: %v", err)
	}

	// TODO(codingllama): Check origin against the public addresses of Proxies and
	//  Auth Servers

	// Accept origins whose host matches the RPID.
	if host == rpID {
		return nil
	}

	// Accept origins whose host is a subdomain of RPID.
	originParts := strings.Split(host, ".")
	rpParts := strings.Split(rpID, ".")
	if len(originParts) <= len(rpParts) {
		return trace.BadParameter("origin doesn't match RPID")
	}
	i := len(originParts) - 1
	j := len(rpParts) - 1
	for j >= 0 {
		if originParts[i] != rpParts[j] {
			return trace.BadParameter("origin doesn't match RPID")
		}
		i--
		j--
	}
	return nil
}
