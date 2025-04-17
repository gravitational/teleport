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

package dynamoevents

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gravitational/trace"
)

// convertLegacyAttributesMap converts a map of legacy (SDK V1) Dynamo attribute
// values into SDK V2 attribute values.
//
// DELETE IN: 19.0.0
func convertLegacyAttributesMap(m map[string]*LegacyAttributeValue) (map[string]types.AttributeValue, error) {
	ret := make(map[string]types.AttributeValue)
	for name, legacyValue := range m {
		val, err := convertLegacyAttributeValue(legacyValue)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ret[name] = val
	}

	return ret, nil
}

// convertLegacyAttributeValue converts a legacy (SDK V1) Dynamo attribute value
// into a newer attribute value (SDK V2).
//
// DELETE IN: 19.0.0
func convertLegacyAttributeValue(legacyAttr *LegacyAttributeValue) (types.AttributeValue, error) {
	switch {
	case legacyAttr.B != nil:
		return &types.AttributeValueMemberB{Value: legacyAttr.B}, nil
	case legacyAttr.BOOL != nil:
		return &types.AttributeValueMemberBOOL{Value: *legacyAttr.BOOL}, nil
	case legacyAttr.BS != nil:
		return &types.AttributeValueMemberBS{Value: legacyAttr.BS}, nil
	case legacyAttr.L != nil:
		attrs := make([]types.AttributeValue, len(legacyAttr.L))
		for i, subAttr := range legacyAttr.L {
			subAttrVal, err := convertLegacyAttributeValue(subAttr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			attrs[i] = subAttrVal
		}

		return &types.AttributeValueMemberL{Value: attrs}, nil
	case legacyAttr.M != nil:
		attrs := make(map[string]types.AttributeValue, len(legacyAttr.M))
		for name, subAttr := range legacyAttr.M {
			subAttrVal, err := convertLegacyAttributeValue(subAttr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			attrs[name] = subAttrVal
		}

		return &types.AttributeValueMemberM{Value: attrs}, nil
	case legacyAttr.N != nil:
		return &types.AttributeValueMemberN{Value: *legacyAttr.N}, nil
	case legacyAttr.NS != nil:
		return &types.AttributeValueMemberNS{Value: aws.ToStringSlice(legacyAttr.NS)}, nil
	case legacyAttr.NULL != nil:
		return &types.AttributeValueMemberNULL{Value: *legacyAttr.NULL}, nil
	case legacyAttr.S != nil:
		return &types.AttributeValueMemberS{Value: *legacyAttr.S}, nil
	case legacyAttr.SS != nil:
		return &types.AttributeValueMemberSS{Value: aws.ToStringSlice(legacyAttr.SS)}, nil
	}

	return nil, trace.BadParameter("unsupported attribute type")
}

// LegacyAttributeValue represents the data for an attribute from AWS SDK V1.
//
// https://github.com/aws/aws-sdk-go/blob/8d203ccff393340d080be0417d091cc60354449b/service/dynamodb/api.go#L8487
//
// DELETE IN: 19.0.0
type LegacyAttributeValue struct {
	// An attribute of type Binary. For example:
	//
	// "B": "dGhpcyB0ZXh0IGlzIGJhc2U2NC1lbmNvZGVk"
	// B is automatically base64 encoded/decoded by the SDK.
	B []byte
	// An attribute of type Boolean. For example:
	//
	// "BOOL": true
	BOOL *bool
	// An attribute of type Binary Set. For example:
	//
	// "BS": ["U3Vubnk=", "UmFpbnk=", "U25vd3k="]
	BS [][]byte
	// An attribute of type List. For example:
	//
	// "L": [ {"S": "Cookies"} , {"S": "Coffee"}, {"N": "3.14159"}]
	L []*LegacyAttributeValue
	// An attribute of type Map. For example:
	//
	// "M": {"Name": {"S": "Joe"}, "Age": {"N": "35"}}
	M map[string]*LegacyAttributeValue
	// An attribute of type Number. For example:
	//
	// "N": "123.45"
	//
	// Numbers are sent across the network to DynamoDB as strings, to maximize compatibility
	// across languages and libraries. However, DynamoDB treats them as number type
	// attributes for mathematical operations.
	N *string
	// An attribute of type Number Set. For example:
	//
	// "NS": ["42.2", "-19", "7.5", "3.14"]
	//
	// Numbers are sent across the network to DynamoDB as strings, to maximize compatibility
	// across languages and libraries. However, DynamoDB treats them as number type
	// attributes for mathematical operations.
	NS []*string
	// An attribute of type Null. For example:
	//
	// "NULL": true
	NULL *bool
	// An attribute of type String. For example:
	//
	// "S": "Hello"
	S *string
	// An attribute of type String Set. For example:
	//
	// "SS": ["Giraffe", "Hippo" ,"Zebra"]
	SS []*string
}
