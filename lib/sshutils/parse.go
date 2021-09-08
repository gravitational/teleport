/*
Copyright 2021 Gravitational, Inc.

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

package sshutils

import (
	"io"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// ParseKnownHosts parses provided known_hosts entries into ssh.PublicKey list.
func ParseKnownHosts(knownHosts [][]byte) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for _, line := range knownHosts {
		for {
			_, _, publicKey, _, bytes, err := ssh.ParseKnownHosts(line)
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, trace.Wrap(err, "failed parsing known hosts: %v; raw line: %q", err, line)
			}
			keys = append(keys, publicKey)
			line = bytes
		}
	}
	return keys, nil
}

// ParseAuthorizedKeys parses provided authorized_keys entries into ssh.PublicKey list.
func ParseAuthorizedKeys(authorizedKeys [][]byte) ([]ssh.PublicKey, error) {
	var keys []ssh.PublicKey
	for _, line := range authorizedKeys {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			return nil, trace.Wrap(err, "failed parsing authorized keys: %v; raw line: %q", err, line)
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}
