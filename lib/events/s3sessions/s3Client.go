/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package s3sessions

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Client interface {
	AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error)
	CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	CreateBucket(ctx context.Context, params *s3.CreateBucketInput, optFns ...func(*s3.Options)) (*s3.CreateBucketOutput, error)
	CreateBucketMetadataTableConfiguration(ctx context.Context, params *s3.CreateBucketMetadataTableConfigurationInput, optFns ...func(*s3.Options)) (*s3.CreateBucketMetadataTableConfigurationOutput, error)
	CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error)
	CreateSession(ctx context.Context, params *s3.CreateSessionInput, optFns ...func(*s3.Options)) (*s3.CreateSessionOutput, error)
	DeleteBucket(ctx context.Context, params *s3.DeleteBucketInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOutput, error)
	DeleteBucketAnalyticsConfiguration(ctx context.Context, params *s3.DeleteBucketAnalyticsConfigurationInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketAnalyticsConfigurationOutput, error)
	DeleteBucketCors(ctx context.Context, params *s3.DeleteBucketCorsInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketCorsOutput, error)
	DeleteBucketEncryption(ctx context.Context, params *s3.DeleteBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketEncryptionOutput, error)
	DeleteBucketIntelligentTieringConfiguration(ctx context.Context, params *s3.DeleteBucketIntelligentTieringConfigurationInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketIntelligentTieringConfigurationOutput, error)
	DeleteBucketInventoryConfiguration(ctx context.Context, params *s3.DeleteBucketInventoryConfigurationInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketInventoryConfigurationOutput, error)
	DeleteBucketLifecycle(ctx context.Context, params *s3.DeleteBucketLifecycleInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketLifecycleOutput, error)
	DeleteBucketMetadataTableConfiguration(ctx context.Context, params *s3.DeleteBucketMetadataTableConfigurationInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketMetadataTableConfigurationOutput, error)
	DeleteBucketMetricsConfiguration(ctx context.Context, params *s3.DeleteBucketMetricsConfigurationInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketMetricsConfigurationOutput, error)
	DeleteBucketOwnershipControls(ctx context.Context, params *s3.DeleteBucketOwnershipControlsInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketOwnershipControlsOutput, error)
	DeleteBucketPolicy(ctx context.Context, params *s3.DeleteBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketPolicyOutput, error)
	DeleteBucketReplication(ctx context.Context, params *s3.DeleteBucketReplicationInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketReplicationOutput, error)
	DeleteBucketTagging(ctx context.Context, params *s3.DeleteBucketTaggingInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketTaggingOutput, error)
	DeleteBucketWebsite(ctx context.Context, params *s3.DeleteBucketWebsiteInput, optFns ...func(*s3.Options)) (*s3.DeleteBucketWebsiteOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	DeleteObjectTagging(ctx context.Context, params *s3.DeleteObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectTaggingOutput, error)
	DeleteObjects(ctx context.Context, params *s3.DeleteObjectsInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
	DeletePublicAccessBlock(ctx context.Context, params *s3.DeletePublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.DeletePublicAccessBlockOutput, error)
	GetBucketAccelerateConfiguration(ctx context.Context, params *s3.GetBucketAccelerateConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketAccelerateConfigurationOutput, error)
	GetBucketAcl(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error)
	GetBucketAnalyticsConfiguration(ctx context.Context, params *s3.GetBucketAnalyticsConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketAnalyticsConfigurationOutput, error)
	GetBucketCors(ctx context.Context, params *s3.GetBucketCorsInput, optFns ...func(*s3.Options)) (*s3.GetBucketCorsOutput, error)
	GetBucketEncryption(ctx context.Context, params *s3.GetBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error)
	GetBucketIntelligentTieringConfiguration(ctx context.Context, params *s3.GetBucketIntelligentTieringConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketIntelligentTieringConfigurationOutput, error)
	GetBucketInventoryConfiguration(ctx context.Context, params *s3.GetBucketInventoryConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketInventoryConfigurationOutput, error)
	GetBucketLifecycleConfiguration(ctx context.Context, params *s3.GetBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error)
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	GetBucketLogging(ctx context.Context, params *s3.GetBucketLoggingInput, optFns ...func(*s3.Options)) (*s3.GetBucketLoggingOutput, error)
	GetBucketMetadataTableConfiguration(ctx context.Context, params *s3.GetBucketMetadataTableConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketMetadataTableConfigurationOutput, error)
	GetBucketMetricsConfiguration(ctx context.Context, params *s3.GetBucketMetricsConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketMetricsConfigurationOutput, error)
	GetBucketNotificationConfiguration(ctx context.Context, params *s3.GetBucketNotificationConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error)
	GetBucketOwnershipControls(ctx context.Context, params *s3.GetBucketOwnershipControlsInput, optFns ...func(*s3.Options)) (*s3.GetBucketOwnershipControlsOutput, error)
	GetBucketPolicy(ctx context.Context, params *s3.GetBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error)
	GetBucketPolicyStatus(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error)
	GetBucketReplication(ctx context.Context, params *s3.GetBucketReplicationInput, optFns ...func(*s3.Options)) (*s3.GetBucketReplicationOutput, error)
	GetBucketRequestPayment(ctx context.Context, params *s3.GetBucketRequestPaymentInput, optFns ...func(*s3.Options)) (*s3.GetBucketRequestPaymentOutput, error)
	GetBucketTagging(ctx context.Context, params *s3.GetBucketTaggingInput, optFns ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error)
	GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
	GetBucketWebsite(ctx context.Context, params *s3.GetBucketWebsiteInput, optFns ...func(*s3.Options)) (*s3.GetBucketWebsiteOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	GetObjectAcl(ctx context.Context, params *s3.GetObjectAclInput, optFns ...func(*s3.Options)) (*s3.GetObjectAclOutput, error)
	GetObjectAttributes(ctx context.Context, params *s3.GetObjectAttributesInput, optFns ...func(*s3.Options)) (*s3.GetObjectAttributesOutput, error)
	GetObjectLegalHold(ctx context.Context, params *s3.GetObjectLegalHoldInput, optFns ...func(*s3.Options)) (*s3.GetObjectLegalHoldOutput, error)
	GetObjectLockConfiguration(ctx context.Context, params *s3.GetObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error)
	GetObjectRetention(ctx context.Context, params *s3.GetObjectRetentionInput, optFns ...func(*s3.Options)) (*s3.GetObjectRetentionOutput, error)
	GetObjectTagging(ctx context.Context, params *s3.GetObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.GetObjectTaggingOutput, error)
	GetObjectTorrent(ctx context.Context, params *s3.GetObjectTorrentInput, optFns ...func(*s3.Options)) (*s3.GetObjectTorrentOutput, error)
	GetPublicAccessBlock(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
	HeadBucket(ctx context.Context, params *s3.HeadBucketInput, optFns ...func(*s3.Options)) (*s3.HeadBucketOutput, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListBucketAnalyticsConfigurations(ctx context.Context, params *s3.ListBucketAnalyticsConfigurationsInput, optFns ...func(*s3.Options)) (*s3.ListBucketAnalyticsConfigurationsOutput, error)
	ListBucketIntelligentTieringConfigurations(ctx context.Context, params *s3.ListBucketIntelligentTieringConfigurationsInput, optFns ...func(*s3.Options)) (*s3.ListBucketIntelligentTieringConfigurationsOutput, error)
	ListBucketInventoryConfigurations(ctx context.Context, params *s3.ListBucketInventoryConfigurationsInput, optFns ...func(*s3.Options)) (*s3.ListBucketInventoryConfigurationsOutput, error)
	ListBucketMetricsConfigurations(ctx context.Context, params *s3.ListBucketMetricsConfigurationsInput, optFns ...func(*s3.Options)) (*s3.ListBucketMetricsConfigurationsOutput, error)
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	ListDirectoryBuckets(ctx context.Context, params *s3.ListDirectoryBucketsInput, optFns ...func(*s3.Options)) (*s3.ListDirectoryBucketsOutput, error)
	ListMultipartUploads(ctx context.Context, params *s3.ListMultipartUploadsInput, optFns ...func(*s3.Options)) (*s3.ListMultipartUploadsOutput, error)
	ListObjectVersions(ctx context.Context, params *s3.ListObjectVersionsInput, optFns ...func(*s3.Options)) (*s3.ListObjectVersionsOutput, error)
	ListObjects(ctx context.Context, params *s3.ListObjectsInput, optFns ...func(*s3.Options)) (*s3.ListObjectsOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	ListParts(ctx context.Context, params *s3.ListPartsInput, optFns ...func(*s3.Options)) (*s3.ListPartsOutput, error)
	Options() s3.Options
	PutBucketAccelerateConfiguration(ctx context.Context, params *s3.PutBucketAccelerateConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketAccelerateConfigurationOutput, error)
	PutBucketAcl(ctx context.Context, params *s3.PutBucketAclInput, optFns ...func(*s3.Options)) (*s3.PutBucketAclOutput, error)
	PutBucketAnalyticsConfiguration(ctx context.Context, params *s3.PutBucketAnalyticsConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketAnalyticsConfigurationOutput, error)
	PutBucketCors(ctx context.Context, params *s3.PutBucketCorsInput, optFns ...func(*s3.Options)) (*s3.PutBucketCorsOutput, error)
	PutBucketEncryption(ctx context.Context, params *s3.PutBucketEncryptionInput, optFns ...func(*s3.Options)) (*s3.PutBucketEncryptionOutput, error)
	PutBucketIntelligentTieringConfiguration(ctx context.Context, params *s3.PutBucketIntelligentTieringConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketIntelligentTieringConfigurationOutput, error)
	PutBucketInventoryConfiguration(ctx context.Context, params *s3.PutBucketInventoryConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketInventoryConfigurationOutput, error)
	PutBucketLifecycleConfiguration(ctx context.Context, params *s3.PutBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketLifecycleConfigurationOutput, error)
	PutBucketLogging(ctx context.Context, params *s3.PutBucketLoggingInput, optFns ...func(*s3.Options)) (*s3.PutBucketLoggingOutput, error)
	PutBucketMetricsConfiguration(ctx context.Context, params *s3.PutBucketMetricsConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketMetricsConfigurationOutput, error)
	PutBucketNotificationConfiguration(ctx context.Context, params *s3.PutBucketNotificationConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutBucketNotificationConfigurationOutput, error)
	PutBucketOwnershipControls(ctx context.Context, params *s3.PutBucketOwnershipControlsInput, optFns ...func(*s3.Options)) (*s3.PutBucketOwnershipControlsOutput, error)
	PutBucketPolicy(ctx context.Context, params *s3.PutBucketPolicyInput, optFns ...func(*s3.Options)) (*s3.PutBucketPolicyOutput, error)
	PutBucketReplication(ctx context.Context, params *s3.PutBucketReplicationInput, optFns ...func(*s3.Options)) (*s3.PutBucketReplicationOutput, error)
	PutBucketRequestPayment(ctx context.Context, params *s3.PutBucketRequestPaymentInput, optFns ...func(*s3.Options)) (*s3.PutBucketRequestPaymentOutput, error)
	PutBucketTagging(ctx context.Context, params *s3.PutBucketTaggingInput, optFns ...func(*s3.Options)) (*s3.PutBucketTaggingOutput, error)
	PutBucketVersioning(ctx context.Context, params *s3.PutBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.PutBucketVersioningOutput, error)
	PutBucketWebsite(ctx context.Context, params *s3.PutBucketWebsiteInput, optFns ...func(*s3.Options)) (*s3.PutBucketWebsiteOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	PutObjectAcl(ctx context.Context, params *s3.PutObjectAclInput, optFns ...func(*s3.Options)) (*s3.PutObjectAclOutput, error)
	PutObjectLegalHold(ctx context.Context, params *s3.PutObjectLegalHoldInput, optFns ...func(*s3.Options)) (*s3.PutObjectLegalHoldOutput, error)
	PutObjectLockConfiguration(ctx context.Context, params *s3.PutObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.PutObjectLockConfigurationOutput, error)
	PutObjectRetention(ctx context.Context, params *s3.PutObjectRetentionInput, optFns ...func(*s3.Options)) (*s3.PutObjectRetentionOutput, error)
	PutObjectTagging(ctx context.Context, params *s3.PutObjectTaggingInput, optFns ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error)
	PutPublicAccessBlock(ctx context.Context, params *s3.PutPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.PutPublicAccessBlockOutput, error)
	RestoreObject(ctx context.Context, params *s3.RestoreObjectInput, optFns ...func(*s3.Options)) (*s3.RestoreObjectOutput, error)
	SelectObjectContent(ctx context.Context, params *s3.SelectObjectContentInput, optFns ...func(*s3.Options)) (*s3.SelectObjectContentOutput, error)
	UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error)
	UploadPartCopy(ctx context.Context, params *s3.UploadPartCopyInput, optFns ...func(*s3.Options)) (*s3.UploadPartCopyOutput, error)
	WriteGetObjectResponse(ctx context.Context, params *s3.WriteGetObjectResponseInput, optFns ...func(*s3.Options)) (*s3.WriteGetObjectResponseOutput, error)
}
