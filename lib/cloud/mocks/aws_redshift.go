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

package mocks

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshift/redshiftiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	libcloudaws "github.com/gravitational/teleport/lib/cloud/aws"
)

// RedshiftMock mocks AWS Redshift API.
type RedshiftMock struct {
	redshiftiface.RedshiftAPI
	Clusters                           []*redshift.Cluster
	GetClusterCredentialsOutput        *redshift.GetClusterCredentialsOutput
	GetClusterCredentialsWithIAMOutput *redshift.GetClusterCredentialsWithIAMOutput
}

func (m *RedshiftMock) GetClusterCredentialsWithContext(aws.Context, *redshift.GetClusterCredentialsInput, ...request.Option) (*redshift.GetClusterCredentialsOutput, error) {
	if m.GetClusterCredentialsOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetClusterCredentialsOutput, nil
}

func (m *RedshiftMock) GetClusterCredentialsWithIAMWithContext(aws.Context, *redshift.GetClusterCredentialsWithIAMInput, ...request.Option) (*redshift.GetClusterCredentialsWithIAMOutput, error) {
	if m.GetClusterCredentialsWithIAMOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetClusterCredentialsWithIAMOutput, nil
}

func (m *RedshiftMock) DescribeClustersWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, options ...request.Option) (*redshift.DescribeClustersOutput, error) {
	if aws.StringValue(input.ClusterIdentifier) == "" {
		return &redshift.DescribeClustersOutput{
			Clusters: m.Clusters,
		}, nil
	}
	for _, cluster := range m.Clusters {
		if aws.StringValue(cluster.ClusterIdentifier) == aws.StringValue(input.ClusterIdentifier) {
			return &redshift.DescribeClustersOutput{
				Clusters: []*redshift.Cluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.StringValue(input.ClusterIdentifier))
}

func (m *RedshiftMock) DescribeClustersPagesWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, fn func(*redshift.DescribeClustersOutput, bool) bool, options ...request.Option) error {
	fn(&redshift.DescribeClustersOutput{
		Clusters: m.Clusters,
	}, true)
	return nil
}

// RedshiftMockUnauth is a mock Redshift client that returns access denied to each call.
type RedshiftMockUnauth struct {
	redshiftiface.RedshiftAPI
}

func (m *RedshiftMockUnauth) DescribeClustersWithContext(ctx aws.Context, input *redshift.DescribeClustersInput, options ...request.Option) (*redshift.DescribeClustersOutput, error) {
	return nil, trace.AccessDenied("unauthorized")
}

// RedshiftGetClusterCredentialsOutput return a sample redshift.GetClusterCredentialsOutput.
func RedshiftGetClusterCredentialsOutput(user, password string, clock clockwork.Clock) *redshift.GetClusterCredentialsOutput {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &redshift.GetClusterCredentialsOutput{
		DbUser:     aws.String(user),
		DbPassword: aws.String(password),
		Expiration: aws.Time(clock.Now().Add(15 * time.Minute)),
	}
}

// RedshiftGetClusterCredentialsWithIAMOutput return a sample
// redshift.GetClusterCredentialsWithIAMeOutput.
func RedshiftGetClusterCredentialsWithIAMOutput(user, password string, clock clockwork.Clock) *redshift.GetClusterCredentialsWithIAMOutput {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &redshift.GetClusterCredentialsWithIAMOutput{
		DbUser:     aws.String(user),
		DbPassword: aws.String(password),
		Expiration: aws.Time(clock.Now().Add(15 * time.Minute)),
	}
}

// RedshiftCluster returns a sample redshift.Cluster.
func RedshiftCluster(name, region string, labels map[string]string, opts ...func(*redshift.Cluster)) *redshift.Cluster {
	cluster := &redshift.Cluster{
		ClusterIdentifier:   aws.String(name),
		ClusterNamespaceArn: aws.String(fmt.Sprintf("arn:aws:redshift:%s:123456789012:namespace:%s", region, name)),
		ClusterStatus:       aws.String("available"),
		Endpoint: &redshift.Endpoint{
			Address: aws.String(fmt.Sprintf("%v.aabbccdd.%v.redshift.amazonaws.com", name, region)),
			Port:    aws.Int64(5439),
		},
		Tags: libcloudaws.LabelsToTags[redshift.Tag](labels),
	}
	for _, opt := range opts {
		opt(cluster)
	}
	return cluster
}
