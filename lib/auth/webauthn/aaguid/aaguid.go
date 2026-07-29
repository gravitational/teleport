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
// build.assets/generate-aaguids.sh. The web UI imports the same file, so both name devices identically.
package aaguid

import (
	"context"
	"embed"
	"encoding/json"
	"sync"

	"github.com/google/uuid"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "WebAuthn")

//go:embed aaguids.json
var embedded embed.FS

var names = sync.OnceValue(func() map[string]string {
	// The table is embedded and parsed by this package's tests, so a failure here means a build shipped a
	// corrupt copy. Logged rather than returned, because the only visible symptom is every device falling
	// back to a generic name, which is otherwise indistinguishable from an authenticator nobody knows.
	// There is no request to attribute this to: it happens once, on the first lookup of the process.
	ctx := context.Background()

	f, err := embedded.ReadFile("aaguids.json")
	if err != nil {
		log.WarnContext(ctx, "Failed to read the embedded AAGUID name table, devices will be named generically", "error", err)

		return nil
	}

	var m map[string]string
	if err := json.Unmarshal(f, &m); err != nil {
		log.WarnContext(ctx, "Failed to parse the embedded AAGUID name table, devices will be named generically", "error", err)

		return nil
	}

	return m
})

// Name returns the make and model reported for an AAGUID, and whether it is known. Authenticators that
// decline to identify themselves report an all-zero AAGUID, which is treated as unknown.
func Name(aaguid uuid.UUID) (string, bool) {
	if aaguid == uuid.Nil {
		return "", false
	}

	name, ok := names()[aaguid.String()]

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

// All returns every known AAGUID and its name. Callers must not modify the returned map.
func All() map[string]string {
	return names()
}
