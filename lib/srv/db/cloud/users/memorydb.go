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

package users

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/memorydb/memorydbiface"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/srv/db/common"
	libsecrets "github.com/gravitational/teleport/lib/srv/db/secrets"
	libutils "github.com/gravitational/teleport/lib/utils"
)

// memoryDBFetcher is a fetcher for discovering MemoryDB users.
type memoryDBFetcher struct {
	cfg   Config
	cache *libutils.FnCache
}

// newMemoryDBFetcher creates a new instance of MemoryDB fetcher.
func newMemoryDBFetcher(cfg Config) (Fetcher, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// cache is used to cache cloud resources fetched from cloud APIs to avoid
	// making the same call repeatedly in a short time.
	cache, err := libutils.NewFnCache(libutils.FnCacheConfig{
		TTL:   cfg.Interval / 2, // Make sure cache expires at next interval.
		Clock: cfg.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &memoryDBFetcher{
		cfg:   cfg,
		cache: cache,
	}, nil
}

// GetType returns the database type of the fetcher. Implements Fetcher.
func (f *memoryDBFetcher) GetType() string {
	return types.DatabaseTypeMemoryDB
}

// FetchDatabaseUsers fetches users for provided database. Implements Fetcher.
func (f *memoryDBFetcher) FetchDatabaseUsers(ctx context.Context, database types.Database) ([]User, error) {
	meta := database.GetAWS()
	if meta.MemoryDB.ACLName == "" {
		return nil, nil
	}

	client, err := f.cfg.Clients.GetAWSMemoryDBClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets, err := newSecretStore(ctx, database, f.cfg.Clients, f.cfg.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	users := []User{}
	mdbUsers, err := f.getManagedUsersForACL(ctx, meta.Region, meta.MemoryDB.ACLName, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, mdbUser := range mdbUsers {
		user, err := f.createUser(mdbUser, client, secrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		users = append(users, user)
	}
	return users, nil
}

// getManagedUsersForACL returns all managed users for specified ACL.
func (f *memoryDBFetcher) getManagedUsersForACL(ctx context.Context, region, aclName string, client memorydbiface.MemoryDBAPI) ([]*memorydb.User, error) {
	allUsers, err := f.getUsersForRegion(ctx, region, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	managedUsers := []*memorydb.User{}
	for _, user := range allUsers {
		// Match ACL.
		if !slices.Contains(aws.StringValueSlice(user.ACLNames), aclName) {
			continue
		}

		// Match special Teleport "managed" tag.
		// If failed to get tags for some users, log the errors instead of failing the function.
		userTags, err := f.getUserTags(ctx, user, client)
		if err != nil {
			if trace.IsAccessDenied(err) {
				f.cfg.Log.DebugContext(ctx, "No Permission to get tags.", "user", aws.StringValue(user.ARN), "error", err)
			} else {
				f.cfg.Log.WarnContext(ctx, "Failed to get tags.", "user", aws.StringValue(user.ARN), "error", err)
			}
			continue
		}
		for _, tag := range userTags {
			if aws.StringValue(tag.Key) == libaws.TagKeyTeleportManaged &&
				libaws.IsTagValueTrue(aws.StringValue(tag.Value)) {
				managedUsers = append(managedUsers, user)
				break
			}
		}
	}
	return managedUsers, nil
}

// getUsersForRegion discovers all MemoryDB users for provided region.
func (f *memoryDBFetcher) getUsersForRegion(ctx context.Context, region string, client memorydbiface.MemoryDBAPI) ([]*memorydb.User, error) {
	getFunc := func(ctx context.Context) ([]*memorydb.User, error) {
		var users []*memorydb.User
		var nextToken *string
		for pageNum := 0; pageNum < common.MaxPages; pageNum++ {
			output, err := client.DescribeUsersWithContext(ctx, &memorydb.DescribeUsersInput{
				NextToken: nextToken,
			})
			if err != nil {
				return nil, trace.Wrap(libaws.ConvertRequestFailureError(err))
			}

			users = append(users, output.Users...)
			if nextToken = output.NextToken; nextToken == nil {
				break
			}
		}
		return users, nil
	}

	users, err := libutils.FnCacheGet(ctx, f.cache, region, getFunc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return users, nil
}

// getUserTags discovers resource tags for provided user.
func (f *memoryDBFetcher) getUserTags(ctx context.Context, user *memorydb.User, client memorydbiface.MemoryDBAPI) ([]*memorydb.Tag, error) {
	getFunc := func(ctx context.Context) ([]*memorydb.Tag, error) {
		output, err := client.ListTagsWithContext(ctx, &memorydb.ListTagsInput{
			ResourceArn: user.ARN,
		})
		if err != nil {
			return nil, trace.Wrap(libaws.ConvertRequestFailureError(err))
		}
		return output.TagList, nil
	}

	userTags, err := libutils.FnCacheGet(ctx, f.cache, aws.StringValue(user.ARN), getFunc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userTags, nil
}

// createUser creates an MemoryDB User.
func (f *memoryDBFetcher) createUser(mdbUser *memorydb.User, client memorydbiface.MemoryDBAPI, secrets libsecrets.Secrets) (User, error) {
	secretKey, err := secretKeyFromAWSARN(aws.StringValue(mdbUser.ARN))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user := &baseUser{
		log:                         f.cfg.Log,
		secretKey:                   secretKey,
		secrets:                     secrets,
		secretTTL:                   f.cfg.Interval,
		databaseUsername:            aws.StringValue(mdbUser.Name),
		clock:                       f.cfg.Clock,
		maxPasswordLength:           128,
		usePreviousPasswordForLogin: true,
		cloudResource: &memoryDBUserResource{
			user:   mdbUser,
			client: client,
		},
	}
	if err := user.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// memoryDBUserResource implements cloudResource interface for a MemoryDB user.
type memoryDBUserResource struct {
	user   *memorydb.User
	client memorydbiface.MemoryDBAPI
}

// ModifyUserPassword updates passwords of an MemoryDB user.
func (r *memoryDBUserResource) ModifyUserPassword(ctx context.Context, oldPassword, newPassword string) error {
	input := &memorydb.UpdateUserInput{
		UserName:           r.user.Name,
		AuthenticationMode: &memorydb.AuthenticationMode{},
	}
	if oldPassword != "" {
		input.AuthenticationMode.Passwords = append(input.AuthenticationMode.Passwords, aws.String(oldPassword))
	}
	if newPassword != "" {
		input.AuthenticationMode.Passwords = append(input.AuthenticationMode.Passwords, aws.String(newPassword))
	}
	if len(input.AuthenticationMode.Passwords) == 0 {
		input.AuthenticationMode.SetType("no-password")
	} else {
		input.AuthenticationMode.SetType("password")
	}

	if _, err := r.client.UpdateUserWithContext(ctx, input); err != nil {
		return trace.Wrap(libaws.ConvertRequestFailureError(err))
	}
	return nil
}
