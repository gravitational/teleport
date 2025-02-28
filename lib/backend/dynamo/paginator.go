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

// QueryPaginator is a paginator for Query
type QueryPaginator struct {
	params    *dynamodb.QueryInput
	client    dynamodb.QueryAPIClient
	nextToken map[string]types.AttributeValue
	firstPage bool
	count     int32
	limit     int32
}

// NewQueryPaginator returns a new QueryPaginator
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

// HasMorePages returns a boolean indicating whether more pages are available
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
