//go:build !pam && cgo
// +build !pam,cgo

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
