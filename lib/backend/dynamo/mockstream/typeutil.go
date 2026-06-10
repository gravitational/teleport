package mockstream

import (
	dynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	streams "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
)

func ToStreamsAV(av dynamodb.AttributeValue) streams.AttributeValue {
	switch v := av.(type) {
	case *dynamodb.AttributeValueMemberS:
		return &streams.AttributeValueMemberS{Value: v.Value}
	case *dynamodb.AttributeValueMemberN:
		return &streams.AttributeValueMemberN{Value: v.Value}
	case *dynamodb.AttributeValueMemberB:
		return &streams.AttributeValueMemberB{Value: v.Value}
	case *dynamodb.AttributeValueMemberBOOL:
		return &streams.AttributeValueMemberBOOL{Value: v.Value}
	case *dynamodb.AttributeValueMemberNULL:
		return &streams.AttributeValueMemberNULL{Value: v.Value}
	case *dynamodb.AttributeValueMemberSS:
		return &streams.AttributeValueMemberSS{Value: v.Value}
	case *dynamodb.AttributeValueMemberNS:
		return &streams.AttributeValueMemberNS{Value: v.Value}
	case *dynamodb.AttributeValueMemberBS:
		return &streams.AttributeValueMemberBS{Value: v.Value}

	case *dynamodb.AttributeValueMemberL:
		out := make([]streams.AttributeValue, len(v.Value))
		for i, item := range v.Value {
			out[i] = ToStreamsAV(item)
		}
		return &streams.AttributeValueMemberL{Value: out}

	case *dynamodb.AttributeValueMemberM:
		out := make(map[string]streams.AttributeValue, len(v.Value))
		for k, val := range v.Value {
			out[k] = ToStreamsAV(val)
		}
		return &streams.AttributeValueMemberM{Value: out}

	default:
		return nil
	}
}

func FromStreamsAV(av streams.AttributeValue) dynamodb.AttributeValue {
	switch v := av.(type) {

	case *streams.AttributeValueMemberS:
		return &dynamodb.AttributeValueMemberS{Value: v.Value}

	case *streams.AttributeValueMemberN:
		return &dynamodb.AttributeValueMemberN{Value: v.Value}

	case *streams.AttributeValueMemberB:
		return &dynamodb.AttributeValueMemberB{Value: v.Value}

	case *streams.AttributeValueMemberBOOL:
		return &dynamodb.AttributeValueMemberBOOL{Value: v.Value}

	case *streams.AttributeValueMemberNULL:
		return &dynamodb.AttributeValueMemberNULL{Value: v.Value}

	case *streams.AttributeValueMemberSS:
		return &dynamodb.AttributeValueMemberSS{Value: v.Value}

	case *streams.AttributeValueMemberNS:
		return &dynamodb.AttributeValueMemberNS{Value: v.Value}

	case *streams.AttributeValueMemberBS:
		return &dynamodb.AttributeValueMemberBS{Value: v.Value}

	case *streams.AttributeValueMemberL:
		out := make([]dynamodb.AttributeValue, len(v.Value))
		for i, item := range v.Value {
			out[i] = FromStreamsAV(item)
		}
		return &dynamodb.AttributeValueMemberL{Value: out}

	case *streams.AttributeValueMemberM:
		out := make(map[string]dynamodb.AttributeValue, len(v.Value))
		for k, val := range v.Value {
			out[k] = FromStreamsAV(val)
		}
		return &dynamodb.AttributeValueMemberM{Value: out}

	default:
		return nil
	}
}

func ToStreamsItem(item map[string]dynamodb.AttributeValue) map[string]streams.AttributeValue {
	out := make(map[string]streams.AttributeValue, len(item))
	for k, v := range item {
		out[k] = ToStreamsAV(v)
	}
	return out
}

func FromStreamsItem(item map[string]streams.AttributeValue) map[string]dynamodb.AttributeValue {
	out := make(map[string]dynamodb.AttributeValue, len(item))
	for k, v := range item {
		out[k] = FromStreamsAV(v)
	}
	return out
}
