/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package aaguid maps WebAuthn AAGUIDs to the make and model of the authenticator that reported them,
// so a freshly registered device can be named after the thing it actually is.
//
// aaguids.json is generated from the community passkey-authenticator-aaguids dataset by
// build.assets/generate-aaguids.sh. The web UI imports the same file, so tsh and the browser name
// devices identically.
package aaguid

import (
	_ "embed"
	"encoding/json"

	"github.com/google/uuid"
)

//go:embed aaguids.json
var embedded []byte

var names = func() map[string]string {
	var m map[string]string
	if err := json.Unmarshal(embedded, &m); err != nil {
		panic("parsing the embedded AAGUID name table: " + err.Error())
	}

	return m
}()

// Name returns the make and model reported for an AAGUID, and whether it is known. Authenticators that
// decline to identify themselves report an all-zero AAGUID, which is treated as unknown.
func Name(aaguid uuid.UUID) (string, bool) {
	if aaguid == uuid.Nil {
		return "", false
	}

	name, ok := names[aaguid.String()]

	return name, ok
}

// NameFromBytes is Name for an AAGUID in its raw 16-byte wire form, as stored on a WebauthnDevice.
func NameFromBytes(aaguid []byte) (string, bool) {
	id, err := uuid.FromBytes(aaguid)
	if err != nil {
		return "", false
	}

	return Name(id)
}
