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
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

func TestParseLegacyDynamoAttributes(t *testing.T) {
	for name, tc := range map[string]struct {
		attributeJSON      string
		expectConvertError require.ErrorAssertionFunc
		expectedAttribute  require.ValueAssertionFunc
	}{
		"binary field": {
			attributeJSON:      `{ "B": "dGVzdAo=", "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberB{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberB)
				// Parsed binaries include line feed character.
				require.Equal(t, []byte("test\n"), attr.Value)
			},
		},
		"bool field": {
			attributeJSON:      `{ "B": null, "BOOL": true, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberBOOL{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberBOOL)
				require.True(t, attr.Value)
			},
		},
		"binary set field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": ["aGVsbG8K", "d29ybGQK"], "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberBS{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberBS)
				// Parsed binaries include line feed character.
				require.ElementsMatch(t, [][]byte{[]byte("hello\n"), []byte("world\n")}, attr.Value)
			},
		},
		"list field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": [{"S": "hello"}, {"S": "world"}], "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberL{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberL)
				require.Len(t, attr.Value, 2)
				require.Equal(t, "hello", attr.Value[0].(*types.AttributeValueMemberS).Value)
				require.Equal(t, "world", attr.Value[1].(*types.AttributeValueMemberS).Value)
			},
		},
		"map field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": {"name": { "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": "test", "SS": null }}, "N": null, "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberM{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberM)
				require.Len(t, attr.Value, 1)
				require.Contains(t, attr.Value, "name")

				mapAttr := attr.Value["name"]
				require.IsType(t, &types.AttributeValueMemberS{}, mapAttr)
				mapAttrS, _ := mapAttr.(*types.AttributeValueMemberS)
				require.Equal(t, "test", mapAttrS.Value)
			},
		},
		"number field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": "123.4", "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberN{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberN)
				require.Equal(t, "123.4", attr.Value)
			},
		},
		"number set field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": ["123", "4.5"], "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberNS{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberNS)
				require.ElementsMatch(t, []string{"123", "4.5"}, attr.Value)
			},
		},
		"null field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": true, "S": null, "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberNULL{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberNULL)
				require.True(t, attr.Value)
			},
		},
		"string field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": "test", "SS": null }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberS{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberS)
				require.Equal(t, "test", attr.Value)
			},
		},
		"string set field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": ["hello", "world"] }`,
			expectConvertError: require.NoError,
			expectedAttribute: func(tt require.TestingT, i1 interface{}, i2 ...interface{}) {
				require.IsType(t, &types.AttributeValueMemberSS{}, i1)
				attr, _ := i1.(*types.AttributeValueMemberSS)
				require.ElementsMatch(t, []string{"hello", "world"}, attr.Value)
			},
		},
		"only null values": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": null }`,
			expectConvertError: require.Error,
			expectedAttribute:  require.Nil,
		},

		"unsupported field": {
			attributeJSON:      `{ "B": null, "BOOL": null, "BS": null, "L": null, "M": null, "N": null, "NS": null, "NULL": null, "S": null, "SS": null, "U": "unsupported" }`,
			expectConvertError: require.Error,
			expectedAttribute:  require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			const attributeTestKey = "test"
			var attr LegacyAttributeValue
			require.NoError(t, json.Unmarshal([]byte(tc.attributeJSON), &attr), "expected a valid legacy attribute JSON definition")

			convertedMap, err := convertLegacyAttributesMap(map[string]*LegacyAttributeValue{attributeTestKey: &attr})
			tc.expectConvertError(t, err)
			tc.expectedAttribute(t, convertedMap[attributeTestKey])
		})
	}
}
