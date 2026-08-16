package mockstream

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/gravitational/trace"
)

// PutItem implements [dynamodb.Client] PutItem for convience in testing.
func (m *MockDynamoStreamsAPI) PutItem(ctx context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if in == nil {
		return nil, trace.BadParameter("PutItemInput is nil")
	}

	if in.TableName == nil || *in.TableName == "" {
		return nil, trace.BadParameter("TableName is required")
	}

	if in.Item == nil {
		return nil, trace.BadParameter("Item is required")
	}

	attr, ok := in.Item[fullPathAttrName]
	if !ok {
		return nil, trace.BadParameter("missing key attribute %q", fullPathAttrName)
	}

	keyAttr, ok := attr.(*dtypes.AttributeValueMemberS)
	if !ok {
		return nil, trace.BadParameter("key attribute %q must be a string", fullPathAttrName)
	}
	key := keyAttr.Value

	rec := streamtypes.Record{
		EventName: streamtypes.OperationTypeInsert,
		Dynamodb: &streamtypes.StreamRecord{
			Keys: map[string]streamtypes.AttributeValue{
				fullPathAttrName: ToStreamsAV(attr),
			},
			NewImage: ToStreamsItem(in.Item),
		},
	}

	h := hashKey(key)
	sh := m.routeShard(h)
	if sh == nil {
		return nil, trace.NotFound("could not route item %q to any shard", key)
	}
	sh.append(rec)
	return &dynamodb.PutItemOutput{}, nil
}

func (m *MockDynamoStreamsAPI) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return &dynamodb.DescribeTableOutput{
		Table: &dtypes.TableDescription{
			LatestStreamArn: aws.String(m.StreamArn),
		},
	}, nil
}
