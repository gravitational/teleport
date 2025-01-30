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
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	ectypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	libaws "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	libsecrets "github.com/gravitational/teleport/lib/srv/db/secrets"
	libutils "github.com/gravitational/teleport/lib/utils"
)

type elasticacheClient interface {
	elasticache.DescribeUsersAPIClient

	ListTagsForResource(ctx context.Context, in *elasticache.ListTagsForResourceInput, optFns ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error)
	ModifyUser(ctx context.Context, in *elasticache.ModifyUserInput, optFns ...func(*elasticache.Options)) (*elasticache.ModifyUserOutput, error)
}

// elastiCacheFetcher is a fetcher for discovering ElastiCache users.
type elastiCacheFetcher struct {
	cfg   Config
	cache *libutils.FnCache
}

// newElastiCacheFetcher creates a new instance of ElastiCache fetcher.
func newElastiCacheFetcher(cfg Config) (Fetcher, error) {
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

	return &elastiCacheFetcher{
		cfg:   cfg,
		cache: cache,
	}, nil
}

// GetType returns the database type of the fetcher. Implements Fetcher.
func (f *elastiCacheFetcher) GetType() string {
	return types.DatabaseTypeElastiCache
}

// FetchDatabaseUsers fetches users for provided database. Implements Fetcher.
func (f *elastiCacheFetcher) FetchDatabaseUsers(ctx context.Context, database types.Database) ([]User, error) {
	meta := database.GetAWS()
	if len(meta.ElastiCache.UserGroupIDs) == 0 {
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

	ecClient := f.cfg.awsClients.getElastiCacheClient(awsCfg)
	users := []User{}
	for _, userGroupID := range meta.ElastiCache.UserGroupIDs {
		managedUsers, err := f.getManagedUsersForGroup(ctx, meta.Region, userGroupID, ecClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, managedUser := range managedUsers {
			user, err := f.createUser(&managedUser, ecClient, secrets)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			users = append(users, user)
		}
	}
	return users, nil
}

// getManagedUsersForGroup returns all managed users for specified user group ID.
func (f *elastiCacheFetcher) getManagedUsersForGroup(ctx context.Context, region, userGroupID string, client elasticacheClient) ([]ectypes.User, error) {
	allUsers, err := f.getUsersForRegion(ctx, region, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	managedUsers := []ectypes.User{}
	for _, user := range allUsers {
		// Match user group ID.
		if !slices.Contains(user.UserGroupIds, userGroupID) {
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

// getUsersForRegion discovers all ElastiCache users for provided region.
func (f *elastiCacheFetcher) getUsersForRegion(ctx context.Context, region string, client elasticacheClient) ([]ectypes.User, error) {
	getFunc := func(ctx context.Context) ([]ectypes.User, error) {
		pager := elasticache.NewDescribeUsersPaginator(client,
			&elasticache.DescribeUsersInput{},
			func(opts *elasticache.DescribeUsersPaginatorOptions) {
				opts.StopOnDuplicateToken = true
			},
		)
		var users []ectypes.User
		for pager.HasMorePages() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, trace.Wrap(libaws.ConvertRequestFailureErrorV2(err))
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
func (f *elastiCacheFetcher) getUserTags(ctx context.Context, user *ectypes.User, client elasticacheClient) ([]ectypes.Tag, error) {
	getFunc := func(ctx context.Context) ([]ectypes.Tag, error) {
		output, err := client.ListTagsForResource(ctx, &elasticache.ListTagsForResourceInput{
			ResourceName: user.ARN,
		})
		if err != nil {
			return nil, trace.Wrap(libaws.ConvertRequestFailureErrorV2(err))
		}
		return output.TagList, nil
	}

	userTags, err := libutils.FnCacheGet(ctx, f.cache, aws.ToString(user.ARN), getFunc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userTags, nil
}

// createUser creates an ElastiCache User.
func (f *elastiCacheFetcher) createUser(ecUser *ectypes.User, client elasticacheClient, secrets libsecrets.Secrets) (User, error) {
	secretKey, err := secretKeyFromAWSARN(aws.ToString(ecUser.ARN))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user := &baseUser{
		log:              f.cfg.Log,
		secretKey:        secretKey,
		secrets:          secrets,
		secretTTL:        f.cfg.Interval,
		databaseUsername: aws.ToString(ecUser.UserName),
		clock:            f.cfg.Clock,

		// Maximum ElastiCache User password size is 128.
		// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth.html
		maxPasswordLength: 128,
		// Both Previous and Current version of the passwords are set to be
		// used for ElastiCache User. Use the Previous version for login in
		// case the Current version is not effective yet while the change is
		// being applied to the user.
		usePreviousPasswordForLogin: true,

		cloudResource: &elastiCacheUserResource{
			user:   ecUser,
			client: client,
		},
	}
	if err := user.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// elastiCacheUserResource implements cloudResource interface for an
// ElastiCache user.
type elastiCacheUserResource struct {
	user   *ectypes.User
	client elasticacheClient
}

// ModifyUserPassword updates passwords of an ElastiCache user.
func (r *elastiCacheUserResource) ModifyUserPassword(ctx context.Context, oldPassword, newPassword string) error {
	passwords := []string{}
	if oldPassword != "" {
		passwords = append(passwords, oldPassword)
	}
	if newPassword != "" {
		passwords = append(passwords, newPassword)
	}

	input := &elasticache.ModifyUserInput{
		UserId:             r.user.UserId,
		Passwords:          passwords,
		NoPasswordRequired: aws.Bool(len(passwords) == 0),
	}
	if _, err := r.client.ModifyUser(ctx, input); err != nil {
		return trace.Wrap(libaws.ConvertRequestFailureErrorV2(err))
	}
	return nil
}
