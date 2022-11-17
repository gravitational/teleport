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

import "github.com/gravitational/trace"

// PingRequest contains the required fields necessary to test a Database Connection.
type PingRequest struct {
	Host     string
	Port     int
	Username string
	Database string
}

// CheckAndSetDefaults validates and set the default values for the PingRequest.
func (req *PingRequest) CheckAndSetDefaults() error {
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
