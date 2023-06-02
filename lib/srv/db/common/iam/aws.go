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

package iam

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

// GetAWSPolicyDocument returns the AWS IAM policy document for provided
// database for setting up the default IAM identity of the database agent.
func GetAWSPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	switch db.GetType() {
	case types.DatabaseTypeRDS, types.DatabaseTypeRDSProxy:
		return getRDSPolicyDocument(db)
	case types.DatabaseTypeRedshift:
		return getRedshiftPolicyDocument(db)
	case types.DatabaseTypeElastiCache:
		return getElastiCachePolicyDocument(db)
	default:
		return nil, nil, trace.BadParameter("GetAWSPolicyDocument is not supported for database type %s", db.GetType())
	}

}

// GetAWSPolicyDocumentForAssumedRole returns the AWS IAM policy document for
// provided database for setting up the IAM role assumed by the database agent.
func GetAWSPolicyDocumentForAssumedRole(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	if db.GetType() == types.DatabaseTypeRedshiftServerless {
		return getRedshiftServerlessPolicyDocument(db)
	}
	return nil, nil, trace.BadParameter("GetAWSPolicyDocumentForAssumedRole is not supported for database type %s", db.GetType())
}

// GetReadableAWSPolicyDocument returns the indented JSON string of the AWS IAM
// policy document for provided database.
func GetReadableAWSPolicyDocument(db types.Database) (string, error) {
	policyDoc, _, err := GetAWSPolicyDocument(db)
	if err != nil {
		return "", trace.Wrap(err)
	}
	marshaled, err := policyDoc.Marshal()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return marshaled, nil
}

// GetReadableAWSPolicyDocumentForAssumedRole returns the indented JSON string
// of the AWS IAM policy document for provided database.
func GetReadableAWSPolicyDocumentForAssumedRole(db types.Database) (string, error) {
	policyDoc, _, err := GetAWSPolicyDocumentForAssumedRole(db)
	if err != nil {
		return "", trace.Wrap(err)
	}
	marshaled, err := policyDoc.Marshal()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return marshaled, nil
}

// CheckElastiCacheSupportsIAMAuth returns whether the given ElastiCache
// database supports IAM auth.
// AWS ElastiCache Redis supports IAM auth for redis version 7+.
func CheckElastiCacheSupportsIAMAuth(database types.Database) (bool, error) {
	version, ok := database.GetLabel("engine-version")
	if !ok {
		return false, trace.NotFound("database missing engine-version label")
	}
	v, err := semver.NewVersion(strings.TrimPrefix(version, "v"))
	if err != nil {
		return false, trace.Wrap(err, "failed to parse engine-version")
	}
	return v.Major >= 7, nil
}

func getRDSPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	resourceID := getRDSResourceID(db)

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&resourceID, "{resource_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"rds-db:connect"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:rds-db:%v:%v:dbuser:%v/*", partition, region, accountID, resourceID),
		},
	})
	return policyDoc, placeholders, nil
}

// getRDSResourceID returns the resource ID for RDS or RDS Proxy database.
func getRDSResourceID(db types.Database) string {
	switch db.GetType() {
	case types.DatabaseTypeRDS:
		return db.GetAWS().RDS.ResourceID
	case types.DatabaseTypeRDSProxy:
		return db.GetAWS().RDSProxy.ResourceID
	default:
		return ""
	}
}

func getRedshiftPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	clusterID := aws.Redshift.ClusterID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&clusterID, "{cluster_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"redshift:GetClusterCredentials"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbuser:%v/*", partition, region, accountID, clusterID),
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbname:%v/*", partition, region, accountID, clusterID),
			fmt.Sprintf("arn:%v:redshift:%v:%v:dbgroup:%v/*", partition, region, accountID, clusterID),
		},
	})
	return policyDoc, placeholders, nil
}

// getRedshiftServerlessPolicyDocument returns the policy document used for
// Redshift Serverless databases.
//
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonredshiftserverless.html
func getRedshiftServerlessPolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	aws := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(aws.Region)
	region := aws.Region
	accountID := aws.AccountID
	workgroupID := aws.RedshiftServerless.WorkgroupID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&workgroupID, "{workgroup_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"redshift-serverless:GetCredentials"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:redshift-serverless:%v:%v:workgroup/%v", partition, region, accountID, workgroupID),
		},
	})
	return policyDoc, placeholders, nil
}

// getElastiCachePolicyDocument returns the policy document used for
// ElastiCache databases.
//
// https://docs.aws.amazon.com/service-authorization/latest/reference/list_amazonelasticache.html
func getElastiCachePolicyDocument(db types.Database) (*awslib.PolicyDocument, Placeholders, error) {
	meta := db.GetAWS()
	partition := awsutils.GetPartitionFromRegion(meta.Region)
	region := meta.Region
	accountID := meta.AccountID
	replicationGroupID := meta.ElastiCache.ReplicationGroupID

	placeholders := Placeholders(nil).
		setPlaceholderIfEmpty(&region, "{region}").
		setPlaceholderIfEmpty(&partition, "{partition}").
		setPlaceholderIfEmpty(&accountID, "{account_id}").
		setPlaceholderIfEmpty(&replicationGroupID, "{replication_group_id}")

	policyDoc := awslib.NewPolicyDocument(&awslib.Statement{
		Effect:  awslib.EffectAllow,
		Actions: awslib.SliceOrString{"elasticache:Connect"},
		Resources: awslib.SliceOrString{
			fmt.Sprintf("arn:%v:elasticache:%v:%v:replicationgroup:%v", partition, region, accountID, replicationGroupID),
			fmt.Sprintf("arn:%v:elasticache:%v:%v:user:*", partition, region, accountID),
		},
	})
	return policyDoc, placeholders, nil
}

// Placeholders defines a slice of strings used as placeholders.
type Placeholders []string

func (s Placeholders) setPlaceholderIfEmpty(attr *string, placeholder string) Placeholders {
	if attr == nil || *attr != "" {
		return s
	}
	*attr = placeholder
	return append(s, placeholder)
}
