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

package msgraphtest

// PayloadListUsers is a fake get application response.
//
// Updating data may require updating integration tests in
// /e/tests/integration/entraid
const PayloadGetApplication = `
{
	"id": "app1",
	"appId": "app1",
	"displayName": "test SAML App",
	"groupMembershipClaims": "SecurityGroup",
	"identifierUris": [
		"goteleport.com"
	],
	"optionalClaims": {
		"accessToken": [],
		"idToken": [],
		"saml2Token": [
			{
				"additionalProperties": [
					"sam_account_name"
				],
				"essential": false,
				"name": "groups",
				"source": null
			}
		]
	}
}`

// PayloadListUsers is a fake list user response.
//
// Updating data may require updating integration tests in
// /e/tests/integration/entraid
const PayloadListUsers = `
[
	{
		"displayName": "Alice Alison",
		"mail": "alice@example.com",
		"userPrincipalName": "alice@example.com",
		"id": "alice@example.com"
	},
	{
		"displayName": "Bob Bobert",
		"givenName": "Bob",
		"jobTitle": "Product Marketing Manager",
		"mail": "bob@example.com",
		"surname": "Bobert",
		"userPrincipalName": "bob@example.com",
		"id": "bob@example.com"
	},
	{
		"businessPhones": [
			"+1 858 555 0110"
		],
		"displayName": "Carol C",
		"givenName": "Carol",
		"jobTitle": "Marketing Assistant",
		"mail": "carol@example.com",
		"officeLocation": "131/1104",
		"preferredLanguage": "en-US",
		"surname": "C",
		"userPrincipalName": "carol@example.com",
		"id": "carol@example.com"
	},
	{
    	"businessPhones": [
    		"8006427676"
    	],
    	"displayName": "Administrator",
    	"givenName": null,
    	"jobTitle": null,
    	"mail": "admin@example.com",
    	"mobilePhone": "5555555555",
    	"officeLocation": null,
    	"preferredLanguage": "en-US",
    	"surname": null,
		"onPremisesSamAccountName": "AD Administrator",
    	"userPrincipalName": "admin@example.com",
    	"id": "5bde3e51-d13b-4db1-9948-fe4b109d11a7"
    },
    {
    	"businessPhones": [
    		"+1 262 555 0106"
    	],
    	"displayName": "Eve Evil",
    	"givenName": "Eve",
    	"jobTitle": "Corporate Security Officer",
    	"mail": "eve@example.com",
    	"mobilePhone": null,
    	"officeLocation": "24/1106",
    	"preferredLanguage": "en-US",
    	"surname": "Evil",
    	"userPrincipalName": "eve#EXT#@example.com",
    	"id": "c03e6eaa-b6ab-46d7-905b-73ec7ea1f755"
    }
]`

// PayloadListUsers is a fake list groups response.
//
// Updating data may require updating integration tests in
// /e/tests/integration/entraid
var PayloadListGroups = `
[
	{
		"id": "group1",
		"displayName": "group1",
		"groupType": ["security-groups"]
	},
	{
		"id": "group2",
		"displayName": "group2",
		"groupType": ["security-groups"]
	},
	{
		"id": "group3",
		"displayName": "group3",
		"groupType": ["security-groups"]
	}
]
`

// PayloadListGroup1Members is a fake list group members response for group1.
//
// Updating data may require updating integration tests in
// /e/tests/integration/entraid
const PayloadListGroup1Members = `
[
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"alice@example.com",
		"mail": "alice@example.com"
	},
	{
		"@odata.type": "#microsoft.graph.device", 
		"id": "1566d9a7-c652-44e7-a75e-665b77431435",
		"mail": "device@example.com"
	},
	{
		"@odata.type": "#microsoft.graph.group",
		"id": "group2",
		"displayName": "group2"
	}
]`

// PayloadListGroup2Members is a fake list group members response for group2.
//
// Updating data may require updating integration tests in
// /e/tests/integration/entraid
const PayloadListGroup2Members = `
[
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"alice@example.com",
		"mail": "alice@example.com"
	},
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"bob@example.com",
		"mail": "bob@example.com"
	},
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"carol@example.com",
		"mail": "carol@example.com"
	}
]`

// PayloadListGroup3Members is a fake list group members response for group3.
//
// Updating data may require updating integration tests in
// /e/tests/integration/entraid
const PayloadListGroup3Members = `
[
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"alice@example.com",
		"mail": "alice@example.com"
	},
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"bob@example.com",
		"mail": "bob@example.com"
	},
	{
		"@odata.type": "#microsoft.graph.user",
		"id":"carol@example.com",
		"mail": "carol@example.com"
	}
]`
