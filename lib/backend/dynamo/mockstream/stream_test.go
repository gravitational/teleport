package mockstream

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"

	"github.com/stretchr/testify/require"
)

func TestStreams(t *testing.T) {
	ctx := t.Context()
	mock := NewMockDynamoStreamsAPI(ctx, Config{
		StreamArn: "mock",
		TableName: "test",
	})
	t.Cleanup(func() { mock.Close() })

	out, err := mock.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String("mock"),
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.StreamDescription.Shards, 1)

	// Verify the root shard has no parent, is open.
	rootShard := out.StreamDescription.Shards[0]
	require.NotEmpty(t, rootShard.ShardId)
	require.Nil(t, rootShard.ParentShardId)
	require.Nil(t, rootShard.SequenceNumberRange.EndingSequenceNumber)

	written := make(map[string]map[string]dtypes.AttributeValue)
	read := make(map[string]map[string]types.AttributeValue)
	for i := range 1024 {
		av := newRandomAv(t, fmt.Sprintf("%d", i))
		key := av[fullPathAttrName].(*dtypes.AttributeValueMemberS).Value
		written[key] = av
		_, err := mock.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("test"),
			Item:      av,
		})
		require.NoError(t, err)

		if i%5 == 0 {
			shards := slices.Collect(mock.GetShards())
			mock.SplitShard(shards[rand.Intn(len(shards))])
		}

		if i%7 == 0 {
			shards := slices.Collect(mock.GetShards())
			mock.SpawnChild(shards[rand.Intn(len(shards))])
		}
	}

	shards, err := mock.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: aws.String("mock"),
	})

	require.NoError(t, err)
	require.NotNil(t, shards)

	for _, sh := range shards.StreamDescription.Shards {
		itOut, err := mock.GetShardIterator(ctx, &dynamodbstreams.GetShardIteratorInput{
			StreamArn:         aws.String("mock"),
			ShardId:           sh.ShardId,
			ShardIteratorType: types.ShardIteratorTypeTrimHorizon,
		})
		require.NoError(t, err)
		require.NotNil(t, itOut)
		it := itOut.ShardIterator
		for it != nil {
			recOut, err := mock.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
				ShardIterator: it,
				Limit:         aws.Int32(50),
			})
			require.NoError(t, err)
			require.NotNil(t, recOut)
			require.LessOrEqual(t, len(recOut.Records), 50)
			for _, rec := range recOut.Records {
				keyAttr, ok := rec.Dynamodb.Keys[fullPathAttrName]
				require.True(t, ok)
				key := keyAttr.(*types.AttributeValueMemberS).Value
				read[key] = rec.Dynamodb.NewImage
			}

			it = recOut.NextShardIterator

			if len(recOut.Records) == 0 {
				break
			}
		}
	}

	require.Equal(t, len(written), len(read))
	for k, v := range written {
		r, ok := read[k]
		require.True(t, ok)
		require.Equal(t, ToStreamsItem(v), r)
	}
}

func newRandomAv(t *testing.T, key string) map[string]dtypes.AttributeValue {
	t.Helper()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	if key == "" {
		key = fmt.Sprintf("sk-%d", r.Intn(1000))
	}

	return map[string]dtypes.AttributeValue{
		fullPathAttrName: &dtypes.AttributeValueMemberS{Value: key},
		"val":            &dtypes.AttributeValueMemberS{Value: fmt.Sprintf("value-%d", r.Intn(1000000))},
		"ts":             &dtypes.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().UnixNano())},
	}

}
