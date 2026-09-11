// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package devicetrust

import "time"

const (
	// PublicEnrollDeviceTimeout is the maximum time the Auth Service allows the
	// enrollment ceremony to take in the public service, from the moment
	// EnrollDevice is called until its handler returns. The ceremony is two round
	// trips, so a healthy client finishes well within it.
	PublicEnrollDeviceTimeout = time.Minute

	// PublicEnrollDeviceProxyTimeout is the timeout for the stream the Proxy
	// Service forwards to the Auth Service. It is longer than
	// PublicEnrollDeviceTimeout so that the Auth Service normally ends the
	// ceremony, tells the caller why and records the timeout in the audit trail.
	// The proxy's timeout only fires if the Auth Service does not answer.
	PublicEnrollDeviceProxyTimeout = PublicEnrollDeviceTimeout + 30*time.Second

	// PublicEnrollDeviceFirstMessageTimeout is how long the Proxy Service waits
	// for the init message. The mobile app holds the enrollment token before it
	// opens the stream, so a healthy client sends init right away.
	PublicEnrollDeviceFirstMessageTimeout = 10 * time.Second
)
