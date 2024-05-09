/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package aws_sync

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

// pollAWSRDSDatabases is a function that returns a function that fetches
// RDS instances and clusters.
func (a *awsFetcher) pollAWSRDSDatabases(ctx context.Context, result *Resources, collectErr func(error)) func() error {
	return func() error {
		var err error
		result.RDSDatabases, err = a.fetchAWSRDSDatabases(ctx)
		if err != nil {
			collectErr(trace.Wrap(err, "failed to fetch databases"))
		}
		return nil
	}
}

// fetchAWSRDSDatabases fetches RDS databases from all regions.
func (a *awsFetcher) fetchAWSRDSDatabases(ctx context.Context) (
	[]*accessgraphv1alpha.AWSRDSDatabaseV1,
	error,
) {
	var (
		dbs     []*accessgraphv1alpha.AWSRDSDatabaseV1
		hostsMu sync.Mutex
		errs    []error
	)
	eG, ctx := errgroup.WithContext(ctx)
	// Set the limit to 5 to avoid too many concurrent requests.
	// This is a temporary solution until we have a better way to limit the
	// number of concurrent requests.
	eG.SetLimit(5)
	collectDBs := func(db *accessgraphv1alpha.AWSRDSDatabaseV1, err error) {
		hostsMu.Lock()
		defer hostsMu.Unlock()
		if err != nil {
			errs = append(errs, err)
		}
		if db != nil {
			dbs = append(dbs, db)
		}

	}

	for _, region := range a.Regions {
		region := region
		eG.Go(func() error {
			rdsClient, err := a.CloudClients.GetAWSRDSClient(ctx, region, a.getAWSOptions()...)
			if err != nil {
				collectDBs(nil, trace.Wrap(err))
				return nil
			}
			err = rdsClient.DescribeDBInstancesPagesWithContext(ctx, &rds.DescribeDBInstancesInput{},
				func(output *rds.DescribeDBInstancesOutput, lastPage bool) bool {
					for _, db := range output.DBInstances {
						// if instance belongs to a cluster, skip it as we want to represent the cluster itself
						// and we pull it using DescribeDBClustersPagesWithContext instead.
						if aws.StringValue(db.DBClusterIdentifier) != "" {
							continue
						}
						protoRDS := awsRDSInstanceToRDS(db, region, a.AccountID)
						collectDBs(protoRDS, nil)
					}
					return !lastPage
				},
			)
			if err != nil {
				collectDBs(nil, trace.Wrap(err))
			}

			err = rdsClient.DescribeDBClustersPagesWithContext(ctx, &rds.DescribeDBClustersInput{},
				func(output *rds.DescribeDBClustersOutput, lastPage bool) bool {
					for _, db := range output.DBClusters {
						protoRDS := awsRDSClusterToRDS(db, region, a.AccountID)
						collectDBs(protoRDS, nil)
					}
					return !lastPage
				},
			)
			if err != nil {
				collectDBs(nil, trace.Wrap(err))
			}

			return nil
		})
	}

	err := eG.Wait()
	return dbs, trace.NewAggregate(append(errs, err)...)
}

// awsRDSInstanceToRDS converts an rds.DBInstance to accessgraphv1alpha.AWSRDSDatabaseV1
// representation.
func awsRDSInstanceToRDS(instance *rds.DBInstance, region, accountID string) *accessgraphv1alpha.AWSRDSDatabaseV1 {
	var tags []*accessgraphv1alpha.AWSTag
	for _, v := range instance.TagList {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.StringValue(v.Key),
			Value: strPtrToWrapper(v.Value),
		})
	}

	return &accessgraphv1alpha.AWSRDSDatabaseV1{
		Name:      aws.StringValue(instance.DBInstanceIdentifier),
		Arn:       aws.StringValue(instance.DBInstanceArn),
		CreatedAt: awsTimeToProtoTime(instance.InstanceCreateTime),
		Status:    aws.StringValue(instance.DBInstanceStatus),
		Region:    region,
		AccountId: accountID,
		Tags:      tags,
		EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
			Engine:  aws.StringValue(instance.Engine),
			Version: aws.StringValue(instance.EngineVersion),
		},
		IsCluster:  false,
		ResourceId: aws.StringValue(instance.DbiResourceId),
	}
}

// awsRDSInstanceToRDS converts an rds.DBCluster to accessgraphv1alpha.AWSRDSDatabaseV1
// representation.
func awsRDSClusterToRDS(instance *rds.DBCluster, region, accountID string) *accessgraphv1alpha.AWSRDSDatabaseV1 {
	var tags []*accessgraphv1alpha.AWSTag
	for _, v := range instance.TagList {
		tags = append(tags, &accessgraphv1alpha.AWSTag{
			Key:   aws.StringValue(v.Key),
			Value: strPtrToWrapper(v.Value),
		})
	}

	return &accessgraphv1alpha.AWSRDSDatabaseV1{
		Name:      aws.StringValue(instance.DBClusterIdentifier),
		Arn:       aws.StringValue(instance.DBClusterArn),
		CreatedAt: awsTimeToProtoTime(instance.ClusterCreateTime),
		Status:    aws.StringValue(instance.Status),
		Region:    region,
		AccountId: accountID,
		Tags:      tags,
		EngineDetails: &accessgraphv1alpha.AWSRDSEngineV1{
			Engine:  aws.StringValue(instance.Engine),
			Version: aws.StringValue(instance.EngineVersion),
		},
		IsCluster:  true,
		ResourceId: aws.StringValue(instance.DbClusterResourceId),
	}
}
