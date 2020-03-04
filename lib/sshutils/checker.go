/*
Copyright 2020 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"

	"golang.org/x/crypto/ssh"
)

// NewCertChecker builds a simple certificate checker which recognizes the
// supplied ssh certificate authorities and obeys FIPS mode restrictions.
func NewCertChecker(cas []ssh.PublicKey) utils.CertChecker {
	// isHostAuthority checks if the supplied ca key matches one of
	// the expected cas.
	isHostAuthority := func(key ssh.PublicKey, addr string) bool {
		for _, caKey := range cas {
			if KeysEqual(key, caKey) {
				return true
			}
		}
		return false
	}
	return utils.CertChecker{
		CertChecker: ssh.CertChecker{
			IsHostAuthority: isHostAuthority,
		},
		FIPS: modules.GetModules().IsBoringBinary(),
	}
}

// NewHostKeyCallback builds an ssh.HostKeyCallback which recognizes the
// supplied ssh certificate authorities and obeys FIPS mode restrictions.
func NewHostKeyCallback(cas []ssh.PublicKey) ssh.HostKeyCallback {
	checker := NewCertChecker(cas)
	return checker.CheckHostKey
}
