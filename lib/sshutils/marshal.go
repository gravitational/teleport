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

package sshutils

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// MarshalAuthorizedKeysFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//	cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func MarshalAuthorizedKeysFormat(clusterName string, keyBytes []byte) (string, error) {
	comment := url.Values{
		"type":        []string{"user"},
		"clustername": []string{clusterName},
	}

	return fmt.Sprintf("cert-authority %s %s\n", strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
}

// KnownHost is a structural representation of a known hosts entry for a Teleport host.
type KnownHost struct {
	AuthorizedKey []byte
	ProxyHost     string
	Hostname      string
	Comment       map[string][]string
}

// MarshalKnownHost returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/known_hosts. The format adheres to the man sshd (8)
// known_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//	@cert-authority proxy.example.com,cluster-a,*.cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func MarshalKnownHost(kh KnownHost) (string, error) {
	if kh.Hostname == "" {
		return "", trace.BadParameter("missing required argument clusterName")
	}

	if len(kh.AuthorizedKey) == 0 {
		return "", trace.BadParameter("missing required argument keyBytes")
	}

	comment := url.Values(kh.Comment)
	if comment == nil {
		comment = url.Values{}
	}

	if _, ok := comment["type"]; !ok {
		comment["type"] = []string{"host"}
	}

	hosts := []string{kh.Hostname, "*." + kh.Hostname}
	if kh.ProxyHost != "" {
		hosts = append([]string{kh.ProxyHost}, hosts...)
	}

	return fmt.Sprintf("@cert-authority %s %s %s\n", strings.Join(hosts, ","), strings.TrimSpace(string(kh.AuthorizedKey)), comment.Encode()), nil
}

// UnmarshalKnownHosts returns a list of authorized hosts from the given known_hosts
// file. Entries in the given file should adhere to the man sshd (8) known_hosts format,
// a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//	@cert-authority proxy.example.com,cluster-a,*.cluster-a ssh-rsa AAA... type=host
//
// UnmarshalKnownHosts will try to guess the proxy host and cluster name for entries that
// look like Teleport authorized host entries, generated with MarshalKnownHost.
func UnmarshalKnownHosts(knownHostsFile [][]byte) ([]KnownHost, error) {
	var knownHosts []KnownHost
	for _, line := range knownHostsFile {
		for {
			_, hosts, publicKey, commentString, rest, err := ssh.ParseKnownHosts(line)
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, trace.Wrap(err, "failed parsing known hosts: %v; raw line: %q", err, line)
			}

			ah := KnownHost{
				AuthorizedKey: ssh.MarshalAuthorizedKey(publicKey),
			}

			comment, err := url.ParseQuery(commentString)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			ah.Comment = map[string][]string(comment)

			// Assuming the known host was generated from MarshalKnownHost,
			// we can get the proxyHost and clusterName for the host.
			switch len(hosts) {
			case 1, 2:
				ah.Hostname = hosts[0]
			case 3:
				ah.ProxyHost = hosts[0]
				ah.Hostname = hosts[1]
			}

			knownHosts = append(knownHosts, ah)

			line = rest
		}
	}

	return knownHosts, nil
}
