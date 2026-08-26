/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package boundkeypair

// Claims contains extended claims resulting from bound keypair joining
type Claims struct {
	// PublicKey is the verified public key trusted at the end of the joining
	// process.
	PublicKey string `json:"public_key"`
	// RecoveryCount is the recovery counter value at the end of the joining
	// process.
	RecoveryCount uint32 `json:"recovery_count"`
	// RecoveryMode is the recovery mode as configured at the time of the join
	// attempt.
	RecoveryMode RecoveryMode `json:"recovery_mode"`
}
