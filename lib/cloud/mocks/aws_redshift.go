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
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/cloud/awstesthelpers"
)

type RedshiftClient struct {
	Unauth bool

	Clusters                           []redshifttypes.Cluster
	GetClusterCredentialsOutput        *redshift.GetClusterCredentialsOutput
	GetClusterCredentialsWithIAMOutput *redshift.GetClusterCredentialsWithIAMOutput
}

func (m *RedshiftClient) DescribeClusters(_ context.Context, input *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	if aws.ToString(input.ClusterIdentifier) == "" {
		return &redshift.DescribeClustersOutput{
			Clusters: m.Clusters,
		}, nil
	}
	for _, cluster := range m.Clusters {
		if aws.ToString(cluster.ClusterIdentifier) == aws.ToString(input.ClusterIdentifier) {
			return &redshift.DescribeClustersOutput{
				Clusters: []redshifttypes.Cluster{cluster},
			}, nil
		}
	}
	return nil, trace.NotFound("cluster %v not found", aws.ToString(input.ClusterIdentifier))
}

func (m *RedshiftClient) GetClusterCredentials(context.Context, *redshift.GetClusterCredentialsInput, ...func(*redshift.Options)) (*redshift.GetClusterCredentialsOutput, error) {
	if m.Unauth || m.GetClusterCredentialsOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetClusterCredentialsOutput, nil
}

func (m *RedshiftClient) GetClusterCredentialsWithIAM(context.Context, *redshift.GetClusterCredentialsWithIAMInput, ...func(*redshift.Options)) (*redshift.GetClusterCredentialsWithIAMOutput, error) {
	if m.Unauth || m.GetClusterCredentialsWithIAMOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetClusterCredentialsWithIAMOutput, nil
}

// RedshiftGetClusterCredentialsOutput return a sample [redshift.GetClusterCredentialsOutput].
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
// [redshift.GetClusterCredentialsWithIAMOutput].
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

func RedshiftCluster(name, region string, labels map[string]string, opts ...func(*redshifttypes.Cluster)) redshifttypes.Cluster {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier:   aws.String(name),
		ClusterNamespaceArn: aws.String(fmt.Sprintf("arn:aws:redshift:%s:123456789012:namespace:%s", region, name)),
		ClusterStatus:       aws.String("available"),
		Endpoint: &redshifttypes.Endpoint{
			Address: aws.String(fmt.Sprintf("%v.aabbccdd.%v.redshift.amazonaws.com", name, region)),
			Port:    aws.Int32(5439),
		},
		Tags: awstesthelpers.LabelsToRedshiftTags(labels),
	}
	for _, opt := range opts {
		opt(&cluster)
	}
	return cluster
}
