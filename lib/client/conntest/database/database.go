/*
Copyright 2022 Gravitational, Inc.

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

package database

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
)

// PingParams contains the required fields necessary to test a Database Connection.
type PingParams struct {
	// Host is the hostname of the Database (does not include port).
	Host string
	// Port is the port where the Database is accepting connections.
	Port int
	// Username is the user to be used to login into the database.
	Username string
	// DatabaseName is an optional database name to be used to login into the database.
	DatabaseName string
}

// CheckAndSetDefaults validates and set the default values for the Ping.
func (p *PingParams) CheckAndSetDefaults(protocol string) error {
	if protocol != defaults.ProtocolMySQL && p.DatabaseName == "" {
		return trace.BadParameter("missing required parameter DatabaseName")
	}

	if p.Username == "" {
		return trace.BadParameter("missing required parameter Username")
	}

	if p.Port == 0 {
		return trace.BadParameter("missing required parameter Port")
	}

	if p.Host == "" {
		p.Host = "localhost"
	}

	return nil
}
