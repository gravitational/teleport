package externalcloudaudit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gravitational/trace"
)

type CreateKMSKeyClient interface {
	// CreateKey creates a unique customer managed KMS key
	CreateKey(ctx context.Context, params *kms.CreateKeyInput, optFns ...func(*kms.Options)) (*kms.CreateKeyOutput, error)
	// Creates a friendly name for a KMS key.
	CreateAlias(ctx context.Context, params *kms.CreateAliasInput, optFns ...func(*kms.Options)) (*kms.CreateAliasOutput, error)
}

type CreateKMSKeyRequest struct {
	// An optional alias for the created kms key
	Alias string
}

type CreateKMSKeyResponse struct {
	// KMSKeyID is ID of created kms key
	KMSKeyID string
}

func CreateKMSKey(ctx context.Context, kmsClient CreateKMSKeyClient, req CreateKMSKeyRequest) (*CreateKMSKeyResponse, error) {
	createOutput, err := kmsClient.CreateKey(ctx, &kms.CreateKeyInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Alias != "" {
		_, err = kmsClient.CreateAlias(ctx, &kms.CreateAliasInput{
			AliasName:   aws.String(req.Alias),
			TargetKeyId: createOutput.KeyMetadata.KeyId,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &CreateKMSKeyResponse{
		KMSKeyID: *createOutput.KeyMetadata.KeyId,
	}, nil
}

// CreateS3BucketClient describes the required methods to create and configure s3 buckets for external cloud audit
type CreateS3BucketClient interface {
	// CreateBucket creates a new S3 bucket.
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	// PutBucketEncryption configures encryption and Amazon S3 Bucket Keys for an existing bucket.
	PutBucketEncryption(ctx context.Context, params *s3.PutBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.PutBucketEncryptionOutput, error)
}

// CreateS3BucketRequest are the request parameters for creating the external cloud audit s3 buckets
type CreateS3BucketRequest struct {
	BucketName string
	Region     string
	KMSKeyID   string
}

// CreateS3Bucket creates the external cloud audit S3 buckets with recommended configurations
func CreateS3Bucket(ctx context.Context, s3Client CreateS3BucketClient, req CreateS3BucketRequest) error {
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(req.BucketName),
		CreateBucketConfiguration: &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(req.Region),
		},
		ObjectOwnership: types.ObjectOwnershipBucketOwnerEnforced,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// If KMSKeyID isn't specified the bucket defaults to SSE-S3
	if req.KMSKeyID != "" {
		_, err = s3Client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
			Bucket: aws.String(req.BucketName),
			ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
				Rules: []types.ServerSideEncryptionRule{
					{
						BucketKeyEnabled: true,
						ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
							SSEAlgorithm:   types.ServerSideEncryptionAwsKms,
							KMSMasterKeyID: aws.String(req.KMSKeyID),
						},
					},
				},
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
