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
	"slices"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/aws/aws-sdk-go/service/opensearchservice/opensearchserviceiface"
	"github.com/gravitational/trace"
)

type OpenSearchMock struct {
	opensearchserviceiface.OpenSearchServiceAPI

	Unauth    bool
	Domains   []*opensearchservice.DomainStatus
	TagsByARN map[string][]*opensearchservice.Tag
}

func (o *OpenSearchMock) ListDomainNamesWithContext(aws.Context, *opensearchservice.ListDomainNamesInput, ...request.Option) (*opensearchservice.ListDomainNamesOutput, error) {
	if o.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	out := &opensearchservice.ListDomainNamesOutput{}
	for _, domain := range o.Domains {
		out.DomainNames = append(out.DomainNames, &opensearchservice.DomainInfo{
			DomainName: domain.DomainName,
			EngineType: aws.String("OpenSearch"),
		})
	}

	return out, nil
}

func (o *OpenSearchMock) DescribeDomainsWithContext(_ aws.Context, input *opensearchservice.DescribeDomainsInput, _ ...request.Option) (*opensearchservice.DescribeDomainsOutput, error) {
	if o.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	out := &opensearchservice.DescribeDomainsOutput{}
	for _, domain := range o.Domains {
		if slices.ContainsFunc(input.DomainNames, func(other *string) bool {
			return aws.StringValue(other) == aws.StringValue(domain.DomainName)
		}) {
			out.DomainStatusList = append(out.DomainStatusList, domain)
		}
	}
	return out, nil
}

func (o *OpenSearchMock) ListTagsWithContext(_ aws.Context, request *opensearchservice.ListTagsInput, _ ...request.Option) (*opensearchservice.ListTagsOutput, error) {
	if o.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	tags, found := o.TagsByARN[aws.StringValue(request.ARN)]
	if !found {
		return nil, trace.NotFound("tags not found")
	}
	return &opensearchservice.ListTagsOutput{TagList: tags}, nil
}

// OpenSearchDomain returns a sample opensearchservice.DomainStatus.
func OpenSearchDomain(name, region string, opts ...func(status *opensearchservice.DomainStatus)) *opensearchservice.DomainStatus {
	domain := &opensearchservice.DomainStatus{
		ARN:           aws.String(fmt.Sprintf("arn:aws:es:%s:123456789012:domain/%s", region, name)),
		DomainId:      aws.String("123456789012/" + name),
		DomainName:    aws.String(name),
		Created:       aws.Bool(true),
		Deleted:       aws.Bool(false),
		EngineVersion: aws.String("OpenSearch_2.5"),

		Endpoint: aws.String(fmt.Sprintf("search-%s-aaaabbbbcccc4444.%s.es.amazonaws.com", name, region)),
	}

	for _, opt := range opts {
		opt(domain)
	}
	return domain
}

func WithOpenSearchVPCEndpoint(name string) func(*opensearchservice.DomainStatus) {
	return func(status *opensearchservice.DomainStatus) {
		if status.Endpoints == nil {
			status.Endpoints = map[string]*string{}
		}
		status.Endpoints[name] = aws.String(fmt.Sprintf("vpc-%v-%v", name, aws.StringValue(status.Endpoint)))
		status.Endpoint = nil
	}
}

func WithOpenSearchCustomEndpoint(endpoint string) func(*opensearchservice.DomainStatus) {
	return func(status *opensearchservice.DomainStatus) {
		status.DomainEndpointOptions = &opensearchservice.DomainEndpointOptions{
			CustomEndpoint:        aws.String(endpoint),
			CustomEndpointEnabled: aws.Bool(true),
			EnforceHTTPS:          aws.Bool(true),
		}
	}
}
