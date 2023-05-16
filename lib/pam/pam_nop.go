//go:build !pam && cgo
// +build !pam,cgo

/*
Copyright 2018 Gravitational, Inc.

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

package pam

import "github.com/gravitational/teleport/lib/service/servicecfg"

var buildHasPAM, systemHasPAM bool

// PAM is used to create a PAM context and initiate PAM transactions to checks
// the users account and open/close a session.
type PAM struct {
}

// Open creates a PAM context and initiates a PAM transaction to check the
// account and then opens a session.
func Open(config *servicecfg.PAMConfig) (*PAM, error) {
	return &PAM{}, nil
}

// Close will close the session, the PAM context, and release any allocated
// memory.
func (p *PAM) Close() error {
	return nil
}

// Environment returns the PAM environment variables associated with a PAM
// handle.
func (p *PAM) Environment() []string {
	return nil
}

// BuildHasPAM returns true if the binary was build with support for PAM
// compiled in.
func BuildHasPAM() bool {
	return buildHasPAM
}

// SystemHasPAM returns true if the PAM library exists on the system.
func SystemHasPAM() bool {
	return systemHasPAM
}
