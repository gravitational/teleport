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
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	rsstypes "github.com/aws/aws-sdk-go-v2/service/redshiftserverless/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

type RedshiftServerlessClient struct {
	Unauth               bool
	Workgroups           []rsstypes.Workgroup
	Endpoints            []rsstypes.EndpointAccess
	TagsByARN            map[string][]rsstypes.Tag
	GetCredentialsOutput *rss.GetCredentialsOutput
}

func (m RedshiftServerlessClient) GetWorkgroup(_ context.Context, input *rss.GetWorkgroupInput, _ ...func(*rss.Options)) (*rss.GetWorkgroupOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	for _, workgroup := range m.Workgroups {
		if aws.ToString(workgroup.WorkgroupName) == aws.ToString(input.WorkgroupName) {
			return &rss.GetWorkgroupOutput{
				Workgroup: &workgroup,
			}, nil
		}
	}
	return nil, trace.NotFound("workgroup %q not found", aws.ToString(input.WorkgroupName))
}

func (m RedshiftServerlessClient) GetEndpointAccess(_ context.Context, input *rss.GetEndpointAccessInput, _ ...func(*rss.Options)) (*rss.GetEndpointAccessOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, endpoint := range m.Endpoints {
		if aws.ToString(endpoint.EndpointName) == aws.ToString(input.EndpointName) {
			return &rss.GetEndpointAccessOutput{
				Endpoint: &endpoint,
			}, nil
		}
	}
	return nil, trace.NotFound("endpoint %q not found", aws.ToString(input.EndpointName))
}

func (m RedshiftServerlessClient) ListWorkgroups(_ context.Context, input *rss.ListWorkgroupsInput, _ ...func(*rss.Options)) (*rss.ListWorkgroupsOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return &rss.ListWorkgroupsOutput{
		Workgroups: m.Workgroups,
	}, nil
}

func (m RedshiftServerlessClient) ListEndpointAccess(_ context.Context, input *rss.ListEndpointAccessInput, _ ...func(*rss.Options)) (*rss.ListEndpointAccessOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	return &rss.ListEndpointAccessOutput{
		Endpoints: m.Endpoints,
	}, nil
}

func (m RedshiftServerlessClient) ListTagsForResource(_ context.Context, input *rss.ListTagsForResourceInput, _ ...func(*rss.Options)) (*rss.ListTagsForResourceOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if m.TagsByARN == nil {
		return &rss.ListTagsForResourceOutput{}, nil
	}
	return &rss.ListTagsForResourceOutput{
		Tags: m.TagsByARN[aws.ToString(input.ResourceArn)],
	}, nil
}

func (m RedshiftServerlessClient) GetCredentials(context.Context, *rss.GetCredentialsInput, ...func(*rss.Options)) (*rss.GetCredentialsOutput, error) {
	if m.Unauth || m.GetCredentialsOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetCredentialsOutput, nil
}

// RedshiftServerlessWorkgroup returns a sample rsstypes.Workgroup.
func RedshiftServerlessWorkgroup(name, region string) *rsstypes.Workgroup {
	return &rsstypes.Workgroup{
		BaseCapacity: aws.Int32(32),
		ConfigParameters: []rsstypes.ConfigParameter{{
			ParameterKey:   aws.String("max_query_execution_time"),
			ParameterValue: aws.String("14400"),
		}},
		CreationDate: aws.Time(sampleTime),
		Endpoint: &rsstypes.Endpoint{
			Address: aws.String(fmt.Sprintf("%v.123456789012.%v.redshift-serverless.amazonaws.com", name, region)),
			Port:    aws.Int32(5439),
			VpcEndpoints: []rsstypes.VpcEndpoint{{
				VpcEndpointId: aws.String("vpc-endpoint-id"),
				VpcId:         aws.String("vpc-id"),
			}},
		},
		NamespaceName:      aws.String("my-namespace"),
		PubliclyAccessible: aws.Bool(true),
		Status:             rsstypes.WorkgroupStatusAvailable,
		WorkgroupArn:       aws.String(fmt.Sprintf("arn:aws:redshift-serverless:%v:123456789012:workgroup/some-uuid-for-%v", region, name)),
		WorkgroupId:        aws.String(fmt.Sprintf("some-uuid-for-%v", name)),
		WorkgroupName:      aws.String(name),
	}
}

// RedshiftServerlessEndpointAccess returns a sample rsstypes.EndpointAccess.
func RedshiftServerlessEndpointAccess(workgroup *rsstypes.Workgroup, name, region string) *rsstypes.EndpointAccess {
	return &rsstypes.EndpointAccess{
		Address:            aws.String(fmt.Sprintf("%s-endpoint-xxxyyyzzz.123456789012.%s.redshift-serverless.amazonaws.com", name, region)),
		EndpointArn:        aws.String(fmt.Sprintf("arn:aws:redshift-serverless:%s:123456789012:managedvpcendpoint/some-uuid-for-%v", region, name)),
		EndpointCreateTime: aws.Time(sampleTime),
		EndpointName:       aws.String(name),
		EndpointStatus:     aws.String("AVAILABLE"),
		Port:               aws.Int32(5439),
		VpcEndpoint: &rsstypes.VpcEndpoint{
			VpcEndpointId: aws.String("vpce-id"),
			VpcId:         aws.String("vpc-id"),
		},
		WorkgroupName: workgroup.WorkgroupName,
	}
}

// RedshiftServerlessGetCredentialsOutput return a sample redshiftserverless.GetCredentialsOutput.
func RedshiftServerlessGetCredentialsOutput(user, password string, clock clockwork.Clock) *rss.GetCredentialsOutput {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &rss.GetCredentialsOutput{
		DbUser:          aws.String(user),
		DbPassword:      aws.String(password),
		Expiration:      aws.Time(clock.Now().Add(15 * time.Minute)),
		NextRefreshTime: aws.Time(clock.Now().Add(2 * time.Hour)),
	}
}

var sampleTime = time.Unix(1645568542, 0) // 2022-02-22 22:22:22
