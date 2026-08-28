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

package profile

// ConnectProfileFile is a common interface for database connection profiles.
type ConnectProfileFile interface {
	// Upsert saves the provided connection profile.
	Upsert(profile ConnectProfile) error
	// Env returns the specified connection profile as environment variables.
	Env(name string) (map[string]string, error)
	// Delete removes the specified connection profile.
	Delete(name string) error
}

// ConnectProfile represents a database connection profile parameters.
type ConnectProfile struct {
	// Name is the profile name.
	Name string
	// Host is the host to connect to.
	Host string
	// Port is the port number to connect to.
	Port int
	// User is an optional database user name.
	User string
	// Database is an optional database name.
	Database string
	// Insecure is whether to skip certificate validation.
	Insecure bool
	// CACertPath is the CA certificate path.
	CACertPath string
	// CertPath is the client certificate path.
	CertPath string
	// KeyPath is the client key path.
	KeyPath string
}
