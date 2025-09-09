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

package scimsdk

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const oktaJSON = `
{
	"department": "family",
	"email": "vito@corleone-foundation.org",
	"employeeNumber": "1",
	"externalId": "00ub1q9yfsRSfO91a5d7",
	"firstName": "Vito",
	"id": "vito@corleone-foundation.org",
	"lastName": "Corleone",
	"meta": {
	  "resourceType": "User",
	  "created": "2024-01-07T22:57:09Z",
	  "location": "/Users/vito@corleone-foundation.org",
	  "version": "2a30170a-b609-473c-bbbb-2abcef8bcf41"
	},
	"mobilePhone": "555 12345678",
	"organization": "The Corleone Family",
	"primaryPhone": "555 98765432",
	"schemas": [
	  "urn:ietf:params:scim:schemas:core:2.0:User",
	  "teleport",
	  "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	],
	"secondEmail": "don@corleonefamily.com",
	"userName": "vito@corleone-foundation.org",
	"name": {
	  "givenName": "Vito",
	  "familyName": "Corleone"
	},
	"emails": [
	  {
		"primary": true,
		"value": "vito@corleone-foundation.org",
		"type": "work"
	  }
	],
	"title": "Don",
	"displayName": "Vito Corleone",
	"phoneNumbers": [
	  {
		"primary": true,
		"value": "555 98765432",
		"type": "work"
	  }
	],
	"locale": "en-US",
	"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User": {
	  "employeeNumber": "1",
	  "costCenter": "Vito",
	  "organization": "The Corleone Family",
	  "department": "family"
	},
	"groups": []
  }
`

func TestUnmarshalResource(t *testing.T) {
	res, err := UnmarshalResource(bytes.NewReader([]byte(oktaJSON)))
	require.NoError(t, err)

	require.Equal(t, "vito@corleone-foundation.org", res.Id)
	require.Equal(t, "00ub1q9yfsRSfO91a5d7", res.ExternalId)
	require.Contains(t, res.Schemas, "urn:ietf:params:scim:schemas:core:2.0:User")

	require.Equal(t, "User", res.Meta.ResourceType)
	require.Equal(t, "2a30170a-b609-473c-bbbb-2abcef8bcf41", res.Meta.Version)

	require.NotNil(t, res.Meta.Created)
	require.Equal(t, time.Date(2024, 01, 07, 22, 57, 9, 0, time.UTC),
		res.Meta.Created.AsTime())

	require.Nil(t, res.Meta.Modified)

	require.Equal(t, "Don", res.Attributes.Fields["title"].GetStringValue())
}

func TestMarshalResource(t *testing.T) {
	res, err := UnmarshalResource(bytes.NewReader([]byte(oktaJSON)))
	require.NoError(t, err)

	body, err := MarshalResource(res)
	require.NoError(t, err)

	var dst AttributeSet
	err = json.Unmarshal(body, &dst)
	require.NoError(t, err, "%#v", err)
	require.Equal(t, "vito@corleone-foundation.org", dst[AttributeID])
	require.Equal(t, "00ub1q9yfsRSfO91a5d7", dst[AttributeExternalID])
	require.Contains(t, dst[AttributeSchemas], "urn:ietf:params:scim:schemas:core:2.0:User")

	meta := dst[AttributeMeta].(map[string]any)
	require.Equal(t, "User", meta["resourceType"])
	require.Equal(t, "2a30170a-b609-473c-bbbb-2abcef8bcf41", meta["version"])
	require.Equal(t, "2024-01-07T22:57:09Z", meta["created"])
	require.Equal(t, nil, meta["modified"])

	require.Equal(t, "Don", dst["title"])
}

func TestMarshalResourceEmpty(t *testing.T) {
	_, err := UnmarshalResource(bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
}
