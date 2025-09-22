// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dynamoathenamigration

import (
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// awsAwsjson10_deserializeDocumentAttributeMap is copied from aws-sdk-go-v2/service/dynamodb/deserializers.go
// AWS SDK does not expose fn to convert from dynamo json export format to types.AttributeValue.
func awsAwsjson10_deserializeDocumentAttributeMap(v *map[string]types.AttributeValue, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var mv map[string]types.AttributeValue
	if *v == nil {
		mv = map[string]types.AttributeValue{}
	} else {
		mv = *v
	}

	for key, value := range shape {
		var parsedVal types.AttributeValue
		mapVar := parsedVal
		if err := awsAwsjson10_deserializeDocumentAttributeValue(&mapVar, value); err != nil {
			return err
		}
		parsedVal = mapVar
		mv[key] = parsedVal

	}
	*v = mv
	return nil
}

func awsAwsjson10_deserializeDocumentAttributeValue(v *types.AttributeValue, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var uv types.AttributeValue
loop:
	for key, value := range shape {
		if value == nil {
			continue
		}
		switch key {
		case "B":
			var mv []byte
			if value != nil {
				jtv, ok := value.(string)
				if !ok {
					return fmt.Errorf("expected BinaryAttributeValue to be []byte, got %T instead", value)
				}
				dv, err := base64.StdEncoding.DecodeString(jtv)
				if err != nil {
					return fmt.Errorf("failed to base64 decode BinaryAttributeValue, %w", err)
				}
				mv = dv
			}
			uv = &types.AttributeValueMemberB{Value: mv}
			break loop

		case "BOOL":
			var mv bool
			if value != nil {
				jtv, ok := value.(bool)
				if !ok {
					return fmt.Errorf("expected BooleanAttributeValue to be of type *bool, got %T instead", value)
				}
				mv = jtv
			}
			uv = &types.AttributeValueMemberBOOL{Value: mv}
			break loop

		case "BS":
			var mv [][]byte
			if err := awsAwsjson10_deserializeDocumentBinarySetAttributeValue(&mv, value); err != nil {
				return err
			}
			uv = &types.AttributeValueMemberBS{Value: mv}
			break loop

		case "L":
			var mv []types.AttributeValue
			if err := awsAwsjson10_deserializeDocumentListAttributeValue(&mv, value); err != nil {
				return err
			}
			uv = &types.AttributeValueMemberL{Value: mv}
			break loop

		case "M":
			var mv map[string]types.AttributeValue
			if err := awsAwsjson10_deserializeDocumentMapAttributeValue(&mv, value); err != nil {
				return err
			}
			uv = &types.AttributeValueMemberM{Value: mv}
			break loop

		case "N":
			var mv string
			if value != nil {
				jtv, ok := value.(string)
				if !ok {
					return fmt.Errorf("expected NumberAttributeValue to be of type string, got %T instead", value)
				}
				mv = jtv
			}
			uv = &types.AttributeValueMemberN{Value: mv}
			break loop

		case "NS":
			var mv []string
			if err := awsAwsjson10_deserializeDocumentNumberSetAttributeValue(&mv, value); err != nil {
				return err
			}
			uv = &types.AttributeValueMemberNS{Value: mv}
			break loop

		case "NULL":
			var mv bool
			if value != nil {
				jtv, ok := value.(bool)
				if !ok {
					return fmt.Errorf("expected NullAttributeValue to be of type *bool, got %T instead", value)
				}
				mv = jtv
			}
			uv = &types.AttributeValueMemberNULL{Value: mv}
			break loop

		case "S":
			var mv string
			if value != nil {
				jtv, ok := value.(string)
				if !ok {
					return fmt.Errorf("expected StringAttributeValue to be of type string, got %T instead", value)
				}
				mv = jtv
			}
			uv = &types.AttributeValueMemberS{Value: mv}
			break loop

		case "SS":
			var mv []string
			if err := awsAwsjson10_deserializeDocumentStringSetAttributeValue(&mv, value); err != nil {
				return err
			}
			uv = &types.AttributeValueMemberSS{Value: mv}
			break loop

		default:
			uv = &types.UnknownUnionMember{Tag: key}
			break loop

		}
	}
	*v = uv
	return nil
}

func awsAwsjson10_deserializeDocumentStringSetAttributeValue(v *[]string, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.([]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var cv []string
	if *v == nil {
		cv = []string{}
	} else {
		cv = *v
	}

	for _, value := range shape {
		var col string
		if value != nil {
			jtv, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected StringAttributeValue to be of type string, got %T instead", value)
			}
			col = jtv
		}
		cv = append(cv, col)

	}
	*v = cv
	return nil
}

func awsAwsjson10_deserializeDocumentNumberSetAttributeValue(v *[]string, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.([]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var cv []string
	if *v == nil {
		cv = []string{}
	} else {
		cv = *v
	}

	for _, value := range shape {
		var col string
		if value != nil {
			jtv, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected NumberAttributeValue to be of type string, got %T instead", value)
			}
			col = jtv
		}
		cv = append(cv, col)

	}
	*v = cv
	return nil
}

func awsAwsjson10_deserializeDocumentBinarySetAttributeValue(v *[][]byte, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.([]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var cv [][]byte
	if *v == nil {
		cv = [][]byte{}
	} else {
		cv = *v
	}

	for _, value := range shape {
		var col []byte
		if value != nil {
			jtv, ok := value.(string)
			if !ok {
				return fmt.Errorf("expected BinaryAttributeValue to be []byte, got %T instead", value)
			}
			dv, err := base64.StdEncoding.DecodeString(jtv)
			if err != nil {
				return fmt.Errorf("failed to base64 decode BinaryAttributeValue, %w", err)
			}
			col = dv
		}
		cv = append(cv, col)

	}
	*v = cv
	return nil
}

func awsAwsjson10_deserializeDocumentMapAttributeValue(v *map[string]types.AttributeValue, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var mv map[string]types.AttributeValue
	if *v == nil {
		mv = map[string]types.AttributeValue{}
	} else {
		mv = *v
	}

	for key, value := range shape {
		var parsedVal types.AttributeValue
		mapVar := parsedVal
		if err := awsAwsjson10_deserializeDocumentAttributeValue(&mapVar, value); err != nil {
			return err
		}
		parsedVal = mapVar
		mv[key] = parsedVal

	}
	*v = mv
	return nil
}

func awsAwsjson10_deserializeDocumentListAttributeValue(v *[]types.AttributeValue, value any) error {
	if v == nil {
		return fmt.Errorf("unexpected nil of type %T", v)
	}
	if value == nil {
		return nil
	}

	shape, ok := value.([]any)
	if !ok {
		return fmt.Errorf("unexpected JSON type %v", value)
	}

	var cv []types.AttributeValue
	if *v == nil {
		cv = []types.AttributeValue{}
	} else {
		cv = *v
	}

	for _, value := range shape {
		var col types.AttributeValue
		if err := awsAwsjson10_deserializeDocumentAttributeValue(&col, value); err != nil {
			return err
		}
		cv = append(cv, col)

	}
	*v = cv
	return nil
}
