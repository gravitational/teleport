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

	"github.com/aws/aws-sdk-go-v2/aws"
	memorydb "github.com/aws/aws-sdk-go-v2/service/memorydb"
	memorydbtypes "github.com/aws/aws-sdk-go-v2/service/memorydb/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	"github.com/gravitational/teleport/lib/srv/db/common"
	libsecrets "github.com/gravitational/teleport/lib/srv/db/secrets"
	libutils "github.com/gravitational/teleport/lib/utils"
)

type memoryDBClient interface {
	memorydb.DescribeUsersAPIClient

	ListTags(ctx context.Context, in *memorydb.ListTagsInput, optFns ...func(*memorydb.Options)) (*memorydb.ListTagsOutput, error)
	UpdateUser(ctx context.Context, in *memorydb.UpdateUserInput, optFns ...func(*memorydb.Options)) (*memorydb.UpdateUserOutput, error)
}

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

	awsCfg, err := f.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	smClt := f.cfg.awsClients.getSecretsManagerClient(awsCfg)
	secrets, err := newSecretStore(database.GetSecretStore(), smClt, f.cfg.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt := f.cfg.awsClients.getMemoryDBClient(awsCfg)
	mdbUsers, err := f.getManagedUsersForACL(ctx, meta.Region, meta.MemoryDB.ACLName, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	users := []User{}
	for _, mdbUser := range mdbUsers {
		user, err := f.createUser(&mdbUser, clt, secrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		users = append(users, user)
	}
	return users, nil
}

// getManagedUsersForACL returns all managed users for specified ACL.
func (f *memoryDBFetcher) getManagedUsersForACL(ctx context.Context, region, aclName string, client memoryDBClient) ([]memorydbtypes.User, error) {
	allUsers, err := f.getUsersForRegion(ctx, region, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	managedUsers := []memorydbtypes.User{}
	for _, user := range allUsers {
		// Match ACL.
		if !slices.Contains(user.ACLNames, aclName) {
			continue
		}

		// Match special Teleport "managed" tag.
		// If failed to get tags for some users, log the errors instead of failing the function.
		userTags, err := f.getUserTags(ctx, &user, client)
		if err != nil {
			if trace.IsAccessDenied(err) {
				f.cfg.Log.DebugContext(ctx, "No Permission to get tags.", "user", aws.ToString(user.ARN), "error", err)
			} else {
				f.cfg.Log.WarnContext(ctx, "Failed to get tags.", "user", aws.ToString(user.ARN), "error", err)
			}
			continue
		}
		for _, tag := range userTags {
			if aws.ToString(tag.Key) == libaws.TagKeyTeleportManaged &&
				libaws.IsTagValueTrue(aws.ToString(tag.Value)) {
				managedUsers = append(managedUsers, user)
				break
			}
		}
	}
	return managedUsers, nil
}

// getUsersForRegion discovers all MemoryDB users for provided region.
func (f *memoryDBFetcher) getUsersForRegion(ctx context.Context, region string, client memoryDBClient) ([]memorydbtypes.User, error) {
	getFunc := func(ctx context.Context) ([]memorydbtypes.User, error) {
		pager := memorydb.NewDescribeUsersPaginator(client,
			&memorydb.DescribeUsersInput{},
			func(opts *memorydb.DescribeUsersPaginatorOptions) {
				opts.StopOnDuplicateToken = true
			},
		)
		var users []memorydbtypes.User
		for i := 0; i < common.MaxPages && pager.HasMorePages(); i++ {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, trace.Wrap(libaws.ConvertRequestFailureError(err))
			}
			users = append(users, page.Users...)
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
func (f *memoryDBFetcher) getUserTags(ctx context.Context, user *memorydbtypes.User, client memoryDBClient) ([]memorydbtypes.Tag, error) {
	getFunc := func(ctx context.Context) ([]memorydbtypes.Tag, error) {
		output, err := client.ListTags(ctx, &memorydb.ListTagsInput{
			ResourceArn: user.ARN,
		})
		if err != nil {
			return nil, trace.Wrap(libaws.ConvertRequestFailureError(err))
		}
		return output.TagList, nil
	}

	userTags, err := libutils.FnCacheGet(ctx, f.cache, aws.ToString(user.ARN), getFunc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userTags, nil
}

// createUser creates an MemoryDB User.
func (f *memoryDBFetcher) createUser(mdbUser *memorydbtypes.User, client memoryDBClient, secrets libsecrets.Secrets) (User, error) {
	secretKey, err := secretKeyFromAWSARN(aws.ToString(mdbUser.ARN))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user := &baseUser{
		log:                         f.cfg.Log,
		secretKey:                   secretKey,
		secrets:                     secrets,
		secretTTL:                   f.cfg.Interval,
		databaseUsername:            aws.ToString(mdbUser.Name),
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
	user   *memorydbtypes.User
	client memoryDBClient
}

// ModifyUserPassword updates passwords of an MemoryDB user.
func (r *memoryDBUserResource) ModifyUserPassword(ctx context.Context, oldPassword, newPassword string) error {
	input := &memorydb.UpdateUserInput{
		UserName: r.user.Name,
		AuthenticationMode: &memorydbtypes.AuthenticationMode{
			Type: memorydbtypes.InputAuthenticationTypePassword,
		},
	}
	if oldPassword != "" {
		input.AuthenticationMode.Passwords = append(input.AuthenticationMode.Passwords, oldPassword)
	}
	input.AuthenticationMode.Passwords = append(input.AuthenticationMode.Passwords, newPassword)

	if _, err := r.client.UpdateUser(ctx, input); err != nil {
		return trace.Wrap(libaws.ConvertRequestFailureError(err))
	}
	return nil
}
