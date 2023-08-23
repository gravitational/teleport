/*
Copyright 2020 Gravitational, Inc.

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

package common

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	dtauthz "github.com/gravitational/teleport/lib/devicetrust/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Session combines parameters for a database connection session.
type Session struct {
	// ID is the unique session ID.
	ID string
	// ClusterName is the cluster the database service is a part of.
	ClusterName string
	// HostID is the id of this database server host.
	HostID string
	// Database is the database user is connecting to.
	Database types.Database
	// Identity is the identity of the connecting Teleport user.
	Identity tlsca.Identity
	// Checker is the access checker for the identity.
	Checker services.AccessChecker
	// AutoCreateUser indicates whether the database user should be auto-created.
	AutoCreateUser bool
	// DatabaseUser is the requested database user.
	DatabaseUser string
	// DatabaseName is the requested database name.
	DatabaseName string
	// DatabaseRoles is a list of roles for auto-provisioned users.
	DatabaseRoles []string
	// StartupParameters define initial connection parameters such as date style.
	StartupParameters map[string]string
	// Log is the logger with session specific fields.
	Log logrus.FieldLogger
	// LockTargets is a list of lock targets applicable to this session.
	LockTargets []types.LockTarget
	// AuthContext is the identity context of the user.
	AuthContext *authz.Context
}

// String returns string representation of the session parameters.
func (c *Session) String() string {
	return fmt.Sprintf("db[%v] identity[%v] dbUser[%v] dbName[%v] autoCreate[%v] dbRoles[%v]",
		c.Database.GetName(), c.Identity.Username, c.DatabaseUser, c.DatabaseName,
		c.AutoCreateUser, strings.Join(c.DatabaseRoles, ","))
}

// GetAccessState returns the AccessState based on the underlying
// [services.AccessChecker] and [tlsca.Identity].
func (c *Session) GetAccessState(authPref types.AuthPreference) services.AccessState {
	state := c.Checker.GetAccessState(authPref)
	state.MFAVerified = c.Identity.MFAVerified != ""
	state.EnableDeviceVerification = true
	state.DeviceVerified = dtauthz.IsTLSDeviceVerified(&c.Identity.DeviceExtensions)
	return state
}

// WithUser returns a shallow copy of the session with overridden database user.
func (c *Session) WithUser(user string) *Session {
	copy := *c
	copy.DatabaseUser = user
	return &copy
}
