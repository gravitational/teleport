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
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/aws/aws-sdk-go/service/redshiftserverless/redshiftserverlessiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// RedshiftServerlessMock mocks RedshiftServerless API.
type RedshiftServerlessMock struct {
	redshiftserverlessiface.RedshiftServerlessAPI

	Unauth               bool
	Workgroups           []*redshiftserverless.Workgroup
	Endpoints            []*redshiftserverless.EndpointAccess
	TagsByARN            map[string][]*redshiftserverless.Tag
	GetCredentialsOutput *redshiftserverless.GetCredentialsOutput
}

func (m RedshiftServerlessMock) GetWorkgroupWithContext(_ aws.Context, input *redshiftserverless.GetWorkgroupInput, _ ...request.Option) (*redshiftserverless.GetWorkgroupOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}

	for _, workgroup := range m.Workgroups {
		if aws.StringValue(workgroup.WorkgroupName) == aws.StringValue(input.WorkgroupName) {
			return new(redshiftserverless.GetWorkgroupOutput).SetWorkgroup(workgroup), nil
		}
	}
	return nil, trace.NotFound("workgroup %q not found", aws.StringValue(input.WorkgroupName))
}
func (m RedshiftServerlessMock) GetEndpointAccessWithContext(_ aws.Context, input *redshiftserverless.GetEndpointAccessInput, _ ...request.Option) (*redshiftserverless.GetEndpointAccessOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	for _, endpoint := range m.Endpoints {
		if aws.StringValue(endpoint.EndpointName) == aws.StringValue(input.EndpointName) {
			return new(redshiftserverless.GetEndpointAccessOutput).SetEndpoint(endpoint), nil
		}
	}
	return nil, trace.NotFound("endpoint %q not found", aws.StringValue(input.EndpointName))
}
func (m RedshiftServerlessMock) ListWorkgroupsPagesWithContext(_ aws.Context, input *redshiftserverless.ListWorkgroupsInput, fn func(*redshiftserverless.ListWorkgroupsOutput, bool) bool, _ ...request.Option) error {
	if m.Unauth {
		return trace.AccessDenied("unauthorized")
	}
	fn(&redshiftserverless.ListWorkgroupsOutput{
		Workgroups: m.Workgroups,
	}, true)
	return nil
}
func (m RedshiftServerlessMock) ListEndpointAccessPagesWithContext(_ aws.Context, input *redshiftserverless.ListEndpointAccessInput, fn func(*redshiftserverless.ListEndpointAccessOutput, bool) bool, _ ...request.Option) error {
	if m.Unauth {
		return trace.AccessDenied("unauthorized")
	}
	fn(&redshiftserverless.ListEndpointAccessOutput{
		Endpoints: m.Endpoints,
	}, true)
	return nil
}
func (m RedshiftServerlessMock) ListTagsForResourceWithContext(_ aws.Context, input *redshiftserverless.ListTagsForResourceInput, _ ...request.Option) (*redshiftserverless.ListTagsForResourceOutput, error) {
	if m.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	if m.TagsByARN == nil {
		return &redshiftserverless.ListTagsForResourceOutput{}, nil
	}
	return &redshiftserverless.ListTagsForResourceOutput{
		Tags: m.TagsByARN[aws.StringValue(input.ResourceArn)],
	}, nil
}
func (m RedshiftServerlessMock) GetCredentialsWithContext(aws.Context, *redshiftserverless.GetCredentialsInput, ...request.Option) (*redshiftserverless.GetCredentialsOutput, error) {
	if m.Unauth || m.GetCredentialsOutput == nil {
		return nil, trace.AccessDenied("access denied")
	}
	return m.GetCredentialsOutput, nil
}

// RedshiftServerlessWorkgroup returns a sample redshiftserverless.Workgroup.
func RedshiftServerlessWorkgroup(name, region string) *redshiftserverless.Workgroup {
	return &redshiftserverless.Workgroup{
		BaseCapacity: aws.Int64(32),
		ConfigParameters: []*redshiftserverless.ConfigParameter{{
			ParameterKey:   aws.String("max_query_execution_time"),
			ParameterValue: aws.String("14400"),
		}},
		CreationDate: aws.Time(sampleTime),
		Endpoint: &redshiftserverless.Endpoint{
			Address: aws.String(fmt.Sprintf("%v.123456789012.%v.redshift-serverless.amazonaws.com", name, region)),
			Port:    aws.Int64(5439),
			VpcEndpoints: []*redshiftserverless.VpcEndpoint{{
				VpcEndpointId: aws.String("vpc-endpoint-id"),
				VpcId:         aws.String("vpc-id"),
			}},
		},
		NamespaceName:      aws.String("my-namespace"),
		PubliclyAccessible: aws.Bool(true),
		Status:             aws.String("AVAILABLE"),
		WorkgroupArn:       aws.String(fmt.Sprintf("arn:aws:redshift-serverless:%v:123456789012:workgroup/some-uuid-for-%v", region, name)),
		WorkgroupId:        aws.String(fmt.Sprintf("some-uuid-for-%v", name)),
		WorkgroupName:      aws.String(name),
	}
}

// RedshiftServerlessEndpointAccess returns a sample redshiftserverless.EndpointAccess.
func RedshiftServerlessEndpointAccess(workgroup *redshiftserverless.Workgroup, name, region string) *redshiftserverless.EndpointAccess {
	return &redshiftserverless.EndpointAccess{
		Address:            aws.String(fmt.Sprintf("%s-endpoint-xxxyyyzzz.123456789012.%s.redshift-serverless.amazonaws.com", name, region)),
		EndpointArn:        aws.String(fmt.Sprintf("arn:aws:redshift-serverless:%s:123456789012:managedvpcendpoint/some-uuid-for-%v", region, name)),
		EndpointCreateTime: aws.Time(sampleTime),
		EndpointName:       aws.String(name),
		EndpointStatus:     aws.String("AVAILABLE"),
		Port:               aws.Int64(5439),
		VpcEndpoint: &redshiftserverless.VpcEndpoint{
			VpcEndpointId: aws.String("vpce-id"),
			VpcId:         aws.String("vpc-id"),
		},
		WorkgroupName: workgroup.WorkgroupName,
	}
}

// RedshiftServerlessGetCredentialsOutput return a sample redshiftserverless.GetCredentialsOutput.
func RedshiftServerlessGetCredentialsOutput(user, password string, clock clockwork.Clock) *redshiftserverless.GetCredentialsOutput {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &redshiftserverless.GetCredentialsOutput{
		DbUser:          aws.String(user),
		DbPassword:      aws.String(password),
		Expiration:      aws.Time(clock.Now().Add(15 * time.Minute)),
		NextRefreshTime: aws.Time(clock.Now().Add(2 * time.Hour)),
	}
}

var sampleTime = time.Unix(1645568542, 0) // 2022-02-22 22:22:22
