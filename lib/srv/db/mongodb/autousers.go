/*
Copyright 2023 Gravitational, Inc.

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

package mongodb

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/gravitational/teleport/lib/srv/db/common"
)

type user struct {
	Username   string         `bson:"user"`
	Roles      []userRole     `bson:"roles"`
	CustomData userCustomData `bson:"customData"`
}

type userCustomData struct {
	TeleportAutoUser bool `bson:"teleport-auto-user"`
}

type userRole struct {
	Rolename string `bson:"role"`
	Database string `bson:"db"`
}

func (u *user) sortRoles() {
	slices.SortFunc(u.Roles, compareUserRole)
}

// ActivateUser creates or enables the database user.
func (e *Engine) ActivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	userRoles, err := makeUserRoles(sessionCtx.DatabaseRoles)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := connectAsAdmin(ctx, sessionCtx, e)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Disconnect(ctx)

	e.Log.Infof("Activating MongoDB user %q with roles %v.", sessionCtx.DatabaseUser, sessionCtx.DatabaseRoles)

	user, found, err := e.getUser(ctx, sessionCtx, client)
	switch {
	case err != nil:
		return trace.Wrap(err)
	case !found:
		return trace.Wrap(e.createUser(ctx, sessionCtx, client, userRoles))
	case !user.CustomData.TeleportAutoUser:
		return trace.AlreadyExists("user %q already exists in this MongoDB database and is not managed by Teleport", sessionCtx.DatabaseUser)
	}

	isActive, err := e.isUserActive(ctx, sessionCtx, client)
	switch {
	case err != nil:
		return trace.Wrap(err)

	case isActive:
		if !slices.Equal(user.Roles, userRoles) {
			return trace.CompareFailed("roles for user %q has changed. Please quit all active connections and try again.", sessionCtx.DatabaseUser)
		}
		e.Log.Debugf("User %q is active and roles are the same.", sessionCtx.DatabaseUser)
		return nil

	default:
		return trace.Wrap(e.updateUser(ctx, sessionCtx, client, userRoles, unLockedAuthRestrictions))
	}
}

// DeactivateUser disables the database user.
func (e *Engine) DeactivateUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	e.Log.Infof("Deactivating MongoDB user %q.", sessionCtx.DatabaseUser)

	client, err := connectAsAdmin(ctx, sessionCtx, e)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Disconnect(ctx)

	isActive, err := e.isUserActive(ctx, sessionCtx, client)
	switch {
	case err != nil:
		return trace.Wrap(err)

	case isActive:
		e.Log.Debugf("Failed to deactivate user %q: user has active connections.", sessionCtx.DatabaseUser)
		return nil

	default:
		return trace.Wrap(e.updateUser(ctx, sessionCtx, client, []userRole{}, lockedAuthRestrictions))
	}
}

// DeleteUser deletes the database user.
func (e *Engine) DeleteUser(ctx context.Context, sessionCtx *common.Session) error {
	if sessionCtx.Database.GetAdminUser().Name == "" {
		return trace.BadParameter("Teleport does not have admin user configured for this database")
	}

	e.Log.Infof("Deleting MongoDB user %q.", sessionCtx.DatabaseUser)

	client, err := connectAsAdmin(ctx, sessionCtx, e)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Disconnect(ctx)

	isActive, err := e.isUserActive(ctx, sessionCtx, client)
	switch {
	case err != nil:
		return trace.Wrap(err)

	case isActive:
		e.Log.Debugf("Failed to delete user %q: user has active connections.", sessionCtx.DatabaseUser)
		return nil

	default:
		return trace.Wrap(e.dropUser(ctx, sessionCtx, client))
	}
}

func (e *Engine) isUserActive(ctx context.Context, sessionCtx *common.Session, client adminClient) (bool, error) {
	logrus.Debugf("Checking if user %q is active.", sessionCtx.DatabaseUser)
	var resp struct {
		Inprog []interface{} `bson:"inprog"`
	}

	err := client.Database(adminDatabaseName).RunCommand(ctx, bson.D{
		{Key: "currentOp", Value: true},
		{Key: "$ownOps", Value: false},
		{Key: "$all", Value: true},
		{Key: "effectiveUsers", Value: bson.M{
			"$elemMatch": bson.M{
				"user": x509Username(sessionCtx),
				"db":   externalDatabaseName,
			},
		}},
		{Key: "comment", Value: runCommandComment},
	}).Decode(&resp)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return len(resp.Inprog) > 0, nil
}

func (e *Engine) getUser(ctx context.Context, sessionCtx *common.Session, client adminClient) (*user, bool, error) {
	logrus.Debugf("Getting user info for %q.", sessionCtx.DatabaseUser)
	var resp struct {
		Users []user `bson:"users"`
	}

	err := client.Database(externalDatabaseName).RunCommand(ctx, bson.D{
		{Key: "usersInfo", Value: x509Username(sessionCtx)},
		{Key: "showCustomData", Value: true},
		{Key: "comment", Value: runCommandComment},
	}).Decode(&resp)
	if err != nil {
		return nil, false, trace.Wrap(err)
	}

	switch len(resp.Users) {
	case 0:
		return nil, false, nil
	case 1:
		user := &resp.Users[0]
		user.sortRoles()
		return user, true, nil
	default:
		return nil, false, trace.BadParameter("expect one MongoDB user but got %v", resp.Users)
	}
}

func (e *Engine) createUser(ctx context.Context, sessionCtx *common.Session, client adminClient, userRoles []userRole) error {
	logrus.Debugf("Creating user %q.", sessionCtx.DatabaseUser)
	return trace.Wrap(client.Database(externalDatabaseName).RunCommand(ctx, bson.D{
		{Key: "createUser", Value: x509Username(sessionCtx)},
		{Key: "roles", Value: userRoles},
		{Key: "customData", Value: userCustomData{TeleportAutoUser: true}},
		{Key: "authenticationRestrictions", Value: unLockedAuthRestrictions},
		{Key: "comment", Value: runCommandComment},
	}).Err())
}

func (e *Engine) updateUser(ctx context.Context, sessionCtx *common.Session, client adminClient, userRoles []userRole, authRestrictions authRestrictions) error {
	logrus.Debugf("Updating roles for user %q.", sessionCtx.DatabaseUser)
	return trace.Wrap(client.Database(externalDatabaseName).RunCommand(ctx, bson.D{
		{Key: "updateUser", Value: x509Username(sessionCtx)},
		{Key: "roles", Value: userRoles},
		{Key: "authenticationRestrictions", Value: authRestrictions},
		{Key: "comment", Value: runCommandComment},
	}).Err())
}

func (e *Engine) dropUser(ctx context.Context, sessionCtx *common.Session, client adminClient) error {
	logrus.Debugf("Dropping user %q.", sessionCtx.DatabaseUser)
	return trace.Wrap(client.Database(externalDatabaseName).RunCommand(ctx, bson.D{
		{Key: "dropUser", Value: x509Username(sessionCtx)},
		{Key: "comment", Value: runCommandComment},
	}).Err())
}

func makeUserRoles(roles []string) ([]userRole, error) {
	userRoles := make([]userRole, 0, len(roles))

	for _, role := range roles {
		rolename, database, ok := strings.Cut(role, "@")
		if !ok {
			return nil, trace.BadParameter("expect DynamoDB role in format of <role>@<db> but got %v", role)
		}

		userRoles = append(userRoles, userRole{
			Rolename: rolename,
			Database: database,
		})
	}
	slices.SortFunc(userRoles, compareUserRole)
	return userRoles, nil
}

func compareUserRole(a, b userRole) int {
	if cmpDatabase := cmp.Compare(a.Database, b.Database); cmpDatabase != 0 {
		return cmpDatabase
	}
	return cmp.Compare(a.Rolename, b.Rolename)
}

const (
	// externalDatabaseName is the name of the "$external" database that
	// manages X.509 users.
	externalDatabaseName = "$external"
	// adminDatabaseName is the name of the "admin" database that "currentOp"
	// command runs at.
	adminDatabaseName = "admin"
	// runCommandComment is a comment used in "runCommand" calls to identify
	// the commands are run by Teleport.
	runCommandComment = "by Teleport Database Service"
)

type authRestrictions bson.A

var (
	unLockedAuthRestrictions authRestrictions = authRestrictions{bson.M{
		"clientSource": bson.A{"0.0.0.0/0"},
	}}

	lockedAuthRestrictions authRestrictions = authRestrictions{bson.M{
		"clientSource": bson.A{"0.0.0.0"},
	}}
)
