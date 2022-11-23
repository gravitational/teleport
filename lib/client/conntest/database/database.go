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
)

// PingParams contains the required fields necessary to test a Database Connection.
type PingParams struct {
	// Host is the hostname of the Database (does not include port).
	Host string
	// Port is the port where the Database is accepting connections.
	Port int
	// Username is the user to be used to login into the database.
	Username string
	// Database is the database name to be used to login into the database.
	Database string
}

// CheckAndSetDefaults validates and set the default values for the Ping.
func (req *PingParams) CheckAndSetDefaults() error {
	if req.Database == "" {
		return trace.BadParameter("missing required parameter Database")
	}

	if req.Username == "" {
		return trace.BadParameter("missing required parameter Username")
	}

	if req.Port == 0 {
		return trace.BadParameter("missing required parameter Port")
	}

	if req.Host == "" {
		req.Host = "localhost"
	}

	return nil
}
