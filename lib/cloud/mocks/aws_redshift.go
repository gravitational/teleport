/*
Copyright 2023 Gravitational, Inc.

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
	Clusters                    []*redshift.Cluster
	GetClusterCredentialsOutput *redshift.GetClusterCredentialsOutput
}

func (m *RedshiftMock) GetClusterCredentialsWithContext(aws.Context, *redshift.GetClusterCredentialsInput, ...request.Option) (*redshift.GetClusterCredentialsOutput, error) {
	if m.GetClusterCredentialsOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetClusterCredentialsOutput, nil
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
