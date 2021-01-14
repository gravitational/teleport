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
	// Insecure is whether to skip certificate validation.a
	Insecure bool
	// CACertPath is the CA certificate path.
	CACertPath string
	// CertPath is the client certificate path.
	CertPath string
	// KeyPath is the client key path.
	KeyPath string
}
