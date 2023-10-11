package externalcloudaudit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gravitational/trace"
)

const (
	// defaultRetentionYears defines default object lock retention period
	defaultRetentionYears = 4
)

// EnsureLTSBucketRequest are request parameters to EnsureLTSBucket
type EnsureLTSBucketRequest struct {
	BucketName string
	Region     string
}

// EnsureLTSBucketClient contains methods required to ensure long term storage bucket exists and is configured correctly
type EnsureLTSBucketClient interface {
	// Methods required to check the configuration of the long term storage bucket
	checkLTSBucketClient
	// Methods required to create the long term storage bucket
	createLTSBucketClient
}

// EnsureLTSBucket ensures the long term storage bucket containing sessions and audit events is present and configured properly
// This function checks if a bucket exists and is configured properly and creates one if it doesn't exist
// If a bucket exists and doesn't have recommended settings configured we currently warn the user and return
func EnsureLTSBucket(ctx context.Context, s3Client EnsureLTSBucketClient, req EnsureLTSBucketRequest) error {
	// Check for existing bucket and it's existing configuration
	// Don't return early if the error is bucket not found as that means we want to create it
	err := checkLTSBucket(ctx, s3Client, req.BucketName, req.Region)
	if err == nil || !isNoSuchBucketError(err) {
		return trace.Wrap(err)
	}

	// Create LTS Bucket
	err = createLTSBucket(ctx, s3Client, req.BucketName, req.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type checkLTSBucketClient interface {
	// Gets the Object Lock configuration for a bucket.
	GetObjectLockConfiguration(ctx context.Context, params *s3.GetObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error)
	// Retrieves OwnershipControls for an Amazon S3 bucket.
	GetBucketOwnershipControls(ctx context.Context, params *s3.GetBucketOwnershipControlsInput, optFns ...func(*s3.Options)) (*s3.GetBucketOwnershipControlsOutput, error)
	// Retrieves the PublicAccessBlock configuration for an Amazon S3 bucket.
	GetPublicAccessBlock(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
	// Returns the Region the bucket resides in.
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

// checkLTSBucket checks for an existing bucket comparing its configuration to what's recommended
func checkLTSBucket(ctx context.Context, s3Client checkLTSBucketClient, bucketName string, region string) error {
	// check the region the bucket exists in
	locationOutput, err := s3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: &bucketName})
	if err != nil {
		return trace.Wrap(err)
	}
	if locationOutput.LocationConstraint != types.BucketLocationConstraint(region) {
		return trace.Errorf("%s is not configured for the %s region", bucketName, region)
	}

	// Versioning is required by object lock so this covers checking if versioning is enabled or not
	objectOutput, err := s3Client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{Bucket: &bucketName})
	if err != nil {
		return trace.Wrap(err)
	}
	if objectOutput.ObjectLockConfiguration.ObjectLockEnabled != types.ObjectLockEnabledEnabled {
		// Is this an appropriate error code?
		return trace.CompareFailed("Existing bucket does not have object lock enabled. Please specify a new bucket or enable object lock on existing one.")
	}

	ownershipOutput, err := s3Client.GetBucketOwnershipControls(ctx, &s3.GetBucketOwnershipControlsInput{Bucket: &bucketName})
	if err != nil {
		return trace.Wrap(err)
	}
	if ownershipOutput.OwnershipControls.Rules[0].ObjectOwnership != types.ObjectOwnershipBucketOwnerEnforced {
		return trace.CompareFailed("Existing bucket does not have object ownership set to BucketOwnerEnforced. Please specify a new bucket or modify existing one.")
	}

	// Ensure public access block is enabled
	accessOutput, err := s3Client.GetPublicAccessBlock(ctx, &s3.GetPublicAccessBlockInput{Bucket: &bucketName})
	if err != nil {
		return trace.Wrap(err)
	}
	pab := accessOutput.PublicAccessBlockConfiguration
	if !(pab.BlockPublicAcls && pab.BlockPublicPolicy && pab.IgnorePublicAcls && pab.RestrictPublicBuckets) {
		return trace.Errorf("One of BlockPublicAcls, BlockPublicPolicy, IngorePublicAcls or RestrictPublicBuckets is not true")
	}

	return nil
}

// createLTSBucketClient contains methods needed to create a new LTS bucket
type createLTSBucketClient interface {
	// Creates a new S3 bucket.
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	// Places an Object Lock configuration on the specified bucket.
	PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error)
	// Sets the versioning state of an existing bucket.
	PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error)
}

// Creates a bucket with the given name in the given region.
// Long Term Storage buckets have the following properties:
// * Bucket versioning enabled
// * Object locking enabled with Governance mode and default retention of 4 years
// * Object ownership set to BucketOwnerEnforced
// * Default SSE-S3 encryption
func createLTSBucket(ctx context.Context, s3Client createLTSBucketClient, bucketName string, region string) error {
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &bucketName,
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		},
		ObjectLockEnabledForBucket: true,
		ACL:                        types.BucketCannedACLPrivate,
		ObjectOwnership:            types.ObjectOwnershipBucketOwnerEnforced,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: &bucketName,
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = s3Client.PutObjectLockConfiguration(ctx, &s3.PutObjectLockConfigurationInput{
		Bucket: &bucketName,
		ObjectLockConfiguration: &types.ObjectLockConfiguration{
			ObjectLockEnabled: types.ObjectLockEnabledEnabled,
			Rule: &types.ObjectLockRule{
				DefaultRetention: &types.DefaultRetention{
					Years: defaultRetentionYears,
					// Modification is prohibited without BypassGovernancePermission iam permission
					Mode: types.ObjectLockRetentionModeGovernance,
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
