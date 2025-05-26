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
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	opensearch "github.com/aws/aws-sdk-go-v2/service/opensearch"
	opensearchtypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	"github.com/gravitational/trace"
)

type OpenSearchClient struct {
	Unauth    bool
	Domains   []opensearchtypes.DomainStatus
	TagsByARN map[string][]opensearchtypes.Tag
}

func (o *OpenSearchClient) ListDomainNames(context.Context, *opensearch.ListDomainNamesInput, ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error) {
	if o.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	out := &opensearch.ListDomainNamesOutput{}
	for _, domain := range o.Domains {
		out.DomainNames = append(out.DomainNames, opensearchtypes.DomainInfo{
			DomainName: domain.DomainName,
			EngineType: opensearchtypes.EngineTypeOpenSearch,
		})
	}

	return out, nil
}

func (o *OpenSearchClient) DescribeDomains(_ context.Context, input *opensearch.DescribeDomainsInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	if o.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	// The real API only allows 5 domains at a time:
	// https://github.com/gravitational/teleport/issues/38651
	if len(input.DomainNames) > 5 {
		return nil, trace.BadParameter("Please provide a maximum of 5 domain names to describe.")
	}
	out := &opensearch.DescribeDomainsOutput{}
	for _, domain := range o.Domains {
		if slices.ContainsFunc(input.DomainNames, func(other string) bool {
			return other == aws.ToString(domain.DomainName)
		}) {
			out.DomainStatusList = append(out.DomainStatusList, domain)
		}
	}
	return out, nil
}

func (o *OpenSearchClient) ListTags(_ context.Context, request *opensearch.ListTagsInput, _ ...func(*opensearch.Options)) (*opensearch.ListTagsOutput, error) {
	if o.Unauth {
		return nil, trace.AccessDenied("unauthorized")
	}
	tags, found := o.TagsByARN[aws.ToString(request.ARN)]
	if !found {
		return nil, trace.NotFound("tags not found")
	}
	return &opensearch.ListTagsOutput{TagList: tags}, nil
}

// OpenSearchDomain returns a sample opensearchtypes.DomainStatus.
func OpenSearchDomain(name, region string, opts ...func(status *opensearchtypes.DomainStatus)) *opensearchtypes.DomainStatus {
	domain := &opensearchtypes.DomainStatus{
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

func WithOpenSearchVPCEndpoint(name string) func(*opensearchtypes.DomainStatus) {
	return func(status *opensearchtypes.DomainStatus) {
		if status.Endpoints == nil {
			status.Endpoints = map[string]string{}
		}
		status.Endpoints[name] = fmt.Sprintf("vpc-%v-%v", name, aws.ToString(status.Endpoint))
		status.Endpoint = nil
	}
}

func WithOpenSearchCustomEndpoint(endpoint string) func(*opensearchtypes.DomainStatus) {
	return func(status *opensearchtypes.DomainStatus) {
		status.DomainEndpointOptions = &opensearchtypes.DomainEndpointOptions{
			CustomEndpoint:        aws.String(endpoint),
			CustomEndpointEnabled: aws.Bool(true),
			EnforceHTTPS:          aws.Bool(true),
		}
	}
}
