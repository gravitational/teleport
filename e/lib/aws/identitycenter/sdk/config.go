package sdk

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/organizations"
	"github.com/aws/aws-sdk-go-v2/service/ssoadmin"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// Config defines configuration parameters for AWS Identity Center SDK client.
type Config struct {
	// AWSConfig is an AWS SDK configuration parameter.
	AWSConfig *aws.Config
	// InstanceARN is Identity Center instance ARN. Must be a valid ARN.
	InstanceARN string
	// Clock is the clock used for time-related operations. Defaults to the
	// system clock.
	Clock clockwork.Clock
	// Logger is the slog logger that the client should use. Defaults to the
	// system default logger if not supplied.
	Logger *slog.Logger
}

func (c *Config) checkAndSetDefault() error {
	if c.InstanceARN == "" {
		return trace.BadParameter("instance ARN is required")
	}
	if _, err := arn.Parse(c.InstanceARN); err != nil {
		return trace.BadParameter("instance ARN %q is invalid", c.InstanceARN)
	}
	if c.AWSConfig == nil {
		return trace.BadParameter("AWS config is required")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Logger == nil {
		c.Logger = slog.Default().With(teleport.ComponentKey, eteleport.ComponentAWSICSDK)
	}
	return nil
}

// organizationsClient satisfies aws-sdk-go-v2 organizations.Client.
type organizationsClient interface {
	// ListAccounts lists Identity Center accounts.
	ListAccounts(ctx context.Context, params *organizations.ListAccountsInput, optFns ...func(*organizations.Options)) (*organizations.ListAccountsOutput, error)
}

// ssoAdminClient satisfies aws-sdk-go-v2 ssoadmin.Client.
type ssoAdminClient interface {
	// ListPermissionSetsProvisionedToAccount lists permission sets provisioned for Identity Center account.
	ListPermissionSetsProvisionedToAccount(ctx context.Context, params *ssoadmin.ListPermissionSetsProvisionedToAccountInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsProvisionedToAccountOutput, error)
	// ListPermissionSets lists permission sets available in the Identity Center instance.
	ListPermissionSets(ctx context.Context, params *ssoadmin.ListPermissionSetsInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListPermissionSetsOutput, error)
	// ListAccountAssignmentsForPrincipal lists account assignment for a given principal, which can either be a user or a user group.
	ListAccountAssignmentsForPrincipal(ctx context.Context, params *ssoadmin.ListAccountAssignmentsForPrincipalInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.ListAccountAssignmentsForPrincipalOutput, error)
	// DescribeInstance returns metadata of an Identity Center instance.
	DescribeInstance(ctx context.Context, params *ssoadmin.DescribeInstanceInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeInstanceOutput, error)
	// DescribePermissionSet returns detailed information of a permission set.
	DescribePermissionSet(ctx context.Context, params *ssoadmin.DescribePermissionSetInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribePermissionSetOutput, error)
	// CreateAccountAssignment creates an account assignment.
	CreateAccountAssignment(ctx context.Context, params *ssoadmin.CreateAccountAssignmentInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.CreateAccountAssignmentOutput, error)
	// DescribeAccountAssignmentCreationStatus returns the status of an account assignment creation.
	DescribeAccountAssignmentCreationStatus(ctx context.Context, params *ssoadmin.DescribeAccountAssignmentCreationStatusInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentCreationStatusOutput, error)
	// DeleteAccountAssignment deletes an account assignment.
	DeleteAccountAssignment(ctx context.Context, params *ssoadmin.DeleteAccountAssignmentInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DeleteAccountAssignmentOutput, error)
	// DescribeAccountAssignmentDeletionStatus returns the status of an account assignment deletion.
	DescribeAccountAssignmentDeletionStatus(ctx context.Context, params *ssoadmin.DescribeAccountAssignmentDeletionStatusInput, optFns ...func(*ssoadmin.Options)) (*ssoadmin.DescribeAccountAssignmentDeletionStatusOutput, error)
}

// identityStoreClient satisfies aws-sdk-go-v2 identitystore.Client.
type identityStoreClient interface {
	// ListUsers lists users from an Identity Center instance.
	ListUsers(ctx context.Context, params *identitystore.ListUsersInput, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error)
	// ListGroups lists user groups from an Identity Center instance.
	ListGroups(ctx context.Context, params *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error)
	// ListGroupMemberships lists user group members from an Identity Center instance.
	ListGroupMemberships(ctx context.Context, params *identitystore.ListGroupMembershipsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupMembershipsOutput, error)
}
