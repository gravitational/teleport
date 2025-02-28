// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dynamo

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go/middleware"
)

// QueryPaginator is a paginator for [dynamodb.Query]. This is
// a vendored version of [dynamodb.QueryPaginator] that dynamically
// adjusts the query limit based on the number of items that have
// already been consumed by the paginator.
type QueryPaginator struct {
	params    *dynamodb.QueryInput
	client    dynamodb.QueryAPIClient
	nextToken map[string]types.AttributeValue
	firstPage bool
	count     int32
	limit     int32
}

// NewQueryPaginator returns a new QueryPaginator.
func NewQueryPaginator(client dynamodb.QueryAPIClient, params *dynamodb.QueryInput) *QueryPaginator {
	if params == nil {
		params = &dynamodb.QueryInput{}
	}

	return &QueryPaginator{
		client:    client,
		params:    params,
		firstPage: true,
		nextToken: params.ExclusiveStartKey,
		limit:     aws.ToInt32(params.Limit),
	}
}

// HasMorePages returns a boolean indicating whether more pages are available.
func (p *QueryPaginator) HasMorePages() bool {
	return (p.firstPage || p.nextToken != nil) && (p.limit <= 0 || p.count < p.limit)
}

// NextPage retrieves the next Query page.
func (p *QueryPaginator) NextPage(ctx context.Context, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if !p.HasMorePages() {
		return nil, errors.New("no more pages available")
	}

	params := *p.params
	params.ExclusiveStartKey = p.nextToken

	const defaultPagesize = int32(1000)

	var limit int32
	if p.limit > 0 {
		limit = min(p.limit-p.count, defaultPagesize)
		params.Limit = aws.Int32(limit)
	}

	optFns = append([]func(*dynamodb.Options){addIsPaginatorUserAgent}, optFns...)
	result, err := p.client.Query(ctx, &params, optFns...)
	if err != nil {
		return nil, err
	}
	p.firstPage = false

	p.nextToken = result.LastEvaluatedKey
	p.count += result.Count

	return result, nil
}

func addIsPaginatorUserAgent(o *dynamodb.Options) {
	o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
		ua, err := getOrAddRequestUserAgent(stack)
		if err != nil {
			return err
		}

		ua.AddUserAgentFeature(awsmiddleware.UserAgentFeaturePaginator)
		return nil
	})
}

func getOrAddRequestUserAgent(stack *middleware.Stack) (*awsmiddleware.RequestUserAgent, error) {
	id := (*awsmiddleware.RequestUserAgent)(nil).ID()
	mw, ok := stack.Build.Get(id)
	if !ok {
		mw = awsmiddleware.NewRequestUserAgent()
		if err := stack.Build.Add(mw, middleware.After); err != nil {
			return nil, err
		}
	}

	ua, ok := mw.(*awsmiddleware.RequestUserAgent)
	if !ok {
		return nil, fmt.Errorf("%T for %s middleware did not match expected type", mw, id)
	}

	return ua, nil
}
