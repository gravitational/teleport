package patch

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSCIMPatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		object  string
		patch   string
		want    string
		wantErr bool
	}{
		{
			name: "Invalid_RemoveWithoutPath",
			object: `{
				"emails": [
					{ "value": "user@example.com", "type": "work" }
				]
			}`,
			patch: `{
				"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
				"Operations": [
					{ "op": "remove" }
				]
			}`,
			wantErr: true,
		},
		{
			name:   "UnknownOperation",
			object: `{"userName": "bjensen"}`,
			patch: `{
				"Operations": [ { "op": "invalid", "path": "userName", "value": "new" } ]
			}`,
			wantErr: true,
		},
		{
			name:    "InvalidJSONPatch",
			object:  `{"userName": "bjensen"}`,
			patch:   `{"Operations": [ { "op": "add", "path": "userName", "value": "new" }`, // missing closing brace
			wantErr: true,
		},
		{
			name: "ReplaceTopLevel",
			object: `{
				"name": { "givenName": "Babs", "familyName": "Smith" }
			}`,
			patch: `{
				"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
				"Operations": [ { "op": "replace", "path": "name.familyName", "value": "Jones" } ]
			}`,
			want: `{
				"name": { "givenName": "Babs", "familyName": "Jones" }
			}`,
		},
		{
			name: "MalformedFilterPath",
			object: `{
				"emails": [{ "value": "a@example.com", "type": "work" }]
			}`,
			patch: `{
				"Operations": [ { "op": "remove", "path": "emails[type eq ]" } ]
			}`,
			wantErr: true,
		},
		{
			name:   "AddToRoot",
			object: `{}`,
			patch: `{
				"Operations": [ { "op": "add", "value": { "userName": "bjensen" } } ]
			}`,
			want: `{"userName":"bjensen"}`,
		},
		{
			name: "AddMultiValued",
			object: `{
				"emails": [ {"value": "babs@jensen.org", "type": "work"} ]
			}`,
			patch: `{
				"Operations": [
					{ "op": "add", "path": "emails", "value": [ {"value": "babs@home.org", "type": "home"} ] }
				]
			}`,
			want: `{
				"emails": [
					{"value": "babs@jensen.org", "type": "work"},
					{"value": "babs@home.org", "type": "home"}
				]
			}`,
		},
		{
			name: "ReplaceFiltered",
			object: `{
				"emails": [
					{ "value": "babs@jensen.org", "type": "work" },
					{ "value": "babs@home.org", "type": "home" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "emails[type eq \"work\"].value", "value": "babs@newdomain.com" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "babs@newdomain.com", "type": "work" },
					{ "value": "babs@home.org", "type": "home" }
				]
			}`,
		},
		{
			name: "RemoveSimple",
			object: `{
				"name": { "givenName": "Babs", "middleName": "Q." }
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "name.middleName" }
				]
			}`,
			want: `{
				"name": { "givenName": "Babs" }
			}`,
		},
		{
			name: "ReplaceEntireDocument",
			object: `{
				"userName": "bjensen", "active": true
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "value": { "userName": "jsmith", "active": false } }
				]
			}`,
			want: `{
				"userName": "jsmith", "active": false
			}`,
		},
		{
			name: "RemoveFilteredArrayElementField",
			object: `{
				"phoneNumbers": [
					{ "value": "555-1111", "type": "home" },
					{ "value": "555-2222", "type": "work" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "phoneNumbers[type eq \"work\"].value" }
				]
			}`,
			want: `{
				"phoneNumbers": [
					{ "value": "555-1111", "type": "home" },
					{ "type": "work" }
				]
			}`,
		},
		{
			name:   "AddToNonExistentMultiValued",
			object: `{}`,
			patch: `{
				"Operations": [
					{ "op": "add", "path": "emails", "value": [ { "value": "babs@work.org", "type": "work" } ] }
				]
			}`,
			want: `{
				"emails": [ { "value": "babs@work.org", "type": "work" } ]
			}`,
		},
		{
			name:   "RemoveNonExistentField",
			object: `{ "userName": "bjensen" }`,
			patch: `{
				"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
				"Operations": [
					{ "op": "remove", "path": "nonexistentField" }
				]
			}`,
			want: `{ "userName": "bjensen" }`,
		},
		{
			name: "RemoveAllMembers",
			object: `{
				"displayName": "Group A",
				"members": [
					{ "value": "2819c223-7f76-453a-919d-413861904646", "display": "User A" },
					{ "value": "902c246b-6245-4190-8e05-00816be7344a", "display": "User B" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "members" }
				]
			}`,
			want: `{
				"displayName": "Group A"
			}`,
		},
		{
			name: "RemoveFilteredEmail",
			object: `{
				"emails": [
					{ "value": "john@example.com", "type": "work" },
					{ "value": "john@home.org", "type": "home" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type eq \"work\"]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "john@home.org", "type": "home" }
				]
			}`,
		},
		{
			name: "ReplaceGivenName",
			object: `{
				"name": { "givenName": "Old", "familyName": "Name" }
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "name.givenName", "value": "New" }
				]
			}`,
			want: `{
				"name": { "givenName": "New", "familyName": "Name" }
			}`,
		},
		{
			name:   "AddEntireObject",
			object: `{ "userName": "babs" }`,
			patch: `{
				"Operations": [
					{ "op": "add", "value": { "active": true } }
				]
			}`,
			want: `{
				"userName": "babs",
				"active": true
			}`,
		},
		{
			name:   "ReplaceEntireDocument",
			object: `{ "userName": "babs", "active": true }`,
			patch: `{
				"Operations": [
					{ "op": "replace", "value": { "userName": "brian", "active": false } }
				]
			}`,
			want: `{
				"userName": "brian",
				"active": false
			}`,
		},
		{
			name: "RemoveFilteredEmailWithComplexCondition",
			object: `{
				"emails": [
					{ "value": "john@example.com", "type": "work" },
					{ "value": "john@example2.com", "type": "work" },
					{ "value": "john@example.org", "type": "home" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type eq \"work\" and value ew \"example.com\"]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "john@example2.com", "type": "work" },
					{ "value": "john@example.org", "type": "home" }
				]
			}`,
		},
		{
			name: "AddToArrayWithoutPath",
			object: `{
								"userName": "john"
			}`,
			patch: `{
				"Operations": [
					{
						"op": "add",
						"value": { "emails": [ { "value": "john@example.com", "type": "work" } ] }
					}
				]
			}`,
			want: `{
				"userName": "john",
				"emails": [ { "value": "john@example.com", "type": "work" } ]
			}`,
		},
		{
			name: "ReplaceFilteredNestedField",
			object: `{
				"addresses": [
					{ "type": "work", "streetAddress": "100 Main St", "locality": "Springfield" },
					{ "type": "home", "streetAddress": "123 Elm St", "locality": "Shelbyville" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "addresses[type eq \"home\"].locality", "value": "Capital City" }
				]
			}`,
			want: `{
				"addresses": [
					{ "type": "work", "streetAddress": "100 Main St", "locality": "Springfield" },
					{ "type": "home", "streetAddress": "123 Elm St", "locality": "Capital City" }
				]
			}`,
		},
		{
			name: "ReplaceArrayElementViaFilter",
			object: `{
				"addresses": [
					{ "type": "home", "locality": "Oldtown" },
					{ "type": "work", "locality": "Oldcity" }
				]
			}`,
			patch: `{
				"Operations": [
					{
						"op": "replace",
						"path": "addresses[type eq \"home\"]",
						"value": { "type": "home", "locality": "Newtown", "region": "East" }
					}
				]
			}`,
			want: `{
				"addresses": [
					{ "type": "home", "locality": "Newtown", "region": "East" },
					{ "type": "work", "locality": "Oldcity" }
				]
			}`,
		},
		{
			name: "ReplaceMultipleArrayElementsViaFilter",
			object: `{
				"addresses": [
					{ "type": "home", "locality": "Oldtown" },
					{ "type": "home", "locality": "Oldvillage" },
					{ "type": "work", "locality": "City" }
				]
			}`,
			patch: `{
				"Operations": [
					{
						"op": "replace",
						"path": "addresses[type eq \"home\"]",
						"value": { "type": "home", "locality": "Newplace" }
					}
				]
			}`,
			want: `{
				"addresses": [
					{ "type": "home", "locality": "Newplace" },
					{ "type": "home", "locality": "Newplace" },
					{ "type": "work", "locality": "City" }
				]
			}`,
		},
		{
			name: "ReplaceEntireArray",
			object: `{
				"emails": [
					{ "value": "old@example.com", "type": "work" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "emails", "value": [ { "value": "new@example.com", "type": "home" } ] }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "new@example.com", "type": "home" }
				]
			}`,
		},
		{
			name:   "InvalidFilterPath",
			object: `{ "userName": "babs" }`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type =" }
				]
			}`,
			wantErr: true,
		},
		{
			name:   "TypeMismatchInFilterValue",
			object: `{ "active": true }`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "active", "value": "false" }
				]
			}`,
			want: `{ "active": "false" }`,
		},
		{
			name: "RemoveWithPRFilter",
			object: `{
				"emails": [
					{ "value": "primary@example.com", "primary": true },
					{ "value": "alt@example.com" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[primary pr]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "alt@example.com" }
				]
			}`,
		},
		{
			name: "AddNestedAttributeToObject",
			object: `{
				"name": { "givenName": "John" }
			}`,
			patch: `{
				"Operations": [
					{ "op": "add", "path": "name.familyName", "value": "Doe" }
				]
			}`,
			want: `{
				"name": { "givenName": "John", "familyName": "Doe" }
			}`,
		},
		{
			name: "ReplaceAttributeInArrayObject",
			object: `{
				"emails": [
					{ "value": "old@example.com", "type": "work" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "emails[type eq \"work\"].value", "value": "new@example.com" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "new@example.com", "type": "work" }
				]
			}`,
		},
		{
			name: "RemoveAttributeWithSubAttr",
			object: `{
				"name": {
					"givenName": "Alice",
					"middleName": "L.",
					"familyName": "Smith"
				}
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "name.middleName" }
				]
			}`,
			want: `{
				"name": {
					"givenName": "Alice",
					"familyName": "Smith"
				}
			}`,
		},
		{
			name: "ReplaceEntireArrayWithSingleValue",
			object: `{
				"emails": [
					{ "value": "old@example.com", "type": "work" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "replace", "path": "emails", "value": [
						{ "value": "new@example.com", "type": "home" }
					] }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "new@example.com", "type": "home" }
				]
			}`,
		},
		{
			name: "ReplaceMultipleArrayElementsViaFilter",
			object: `{
				"addresses": [
					{ "type": "home", "locality": "Oldtown" },
					{ "type": "home", "locality": "Oldvillage" },
					{ "type": "work", "locality": "City" }
				]
			}`,
			patch: `{
				"Operations": [
					{
						"op": "replace",
						"path": "addresses[type eq \"home\"]",
						"value": { "type": "home", "locality": "Newplace" }
					}
				]
			}`,
			want: `{
				"addresses": [
					{ "type": "home", "locality": "Newplace" },
					{ "type": "home", "locality": "Newplace" },
					{ "type": "work", "locality": "City" }
				]
			}`,
		},
		{
			name: "ReplaceIntFieldViaFilter",
			object: `{
				"meta": [
					{ "type": "version", "value": 1 },
					{ "type": "build", "value": 2 }
				]
			}`,
			patch: `{
				"Operations": [
					{
						"op": "replace",
						"path": "meta[value eq 2].value",
						"value": 3
					}
				]
			}`,
			want: `{
				"meta": [
					{ "type": "version", "value": 1 },
					{ "type": "build", "value": 3 }
				]
			}`,
		},
		{
			name: "RemoveIntFieldViaFilter",
			object: `{
				"records": [
					{ "id": 1, "name": "Alpha" },
					{ "id": 2, "name": "Beta" }
				]
			}`,
			patch: `{
				"Operations": [
					{
						"op": "remove",
						"path": "records[id eq 1]"
					}
				]
			}`,
			want: `{
				"records": [
					{ "id": 2, "name": "Beta" }
				]
			}`,
		},
		{
			name: "RemoveElementWithNEFilter",
			object: `{
				"emails": [
					{ "value": "a@example.com", "type": "work" },
					{ "value": "b@example.com", "type": "home" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type ne \"work\"]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "a@example.com", "type": "work" }
				]
			}`,
		},
		{
			name: "RemoveWithNotFilter",
			object: `{
				"emails": [
					{ "value": "a@example.com", "primary": true },
					{ "value": "b@example.com" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[not (primary pr)]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "a@example.com", "primary": true }
				]
			}`,
		},
		{
			name: "RemoveWithLogicalOR",
			object: `{
				"emails": [
					{ "value": "a@example.com", "type": "work" },
					{ "value": "b@example.com", "type": "home" },
					{ "value": "c@example.com", "type": "other" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type eq \"home\" or type eq \"other\"]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "a@example.com", "type": "work" }
				]
			}`,
		},
		{
			name: "RemoveSubAttributeOnSomeElementsOnly",
			object: `{
				"phoneNumbers": [
					{ "type": "mobile", "value": "123" },
					{ "type": "home" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "phoneNumbers[type eq \"mobile\"].value" }
				]
			}`,
			want: `{
				"phoneNumbers": [
					{ "type": "mobile" },
					{ "type": "home" }
				]
			}`,
		},
		{
			name: "RemoveNonExistentSubAttr",
			object: `{
				"name": { "givenName": "Alice" }
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "name.middleName" }
				]
			}`,
			want: `{
				"name": { "givenName": "Alice" }
			}`,
		},
		{
			name: "Filter_UserTypeAndEmailsNestedValue",
			object: `{
				"userType": "Employee",
				"emails": [
					{ "value": "bjensen@example.com", "type": "work" },
					{ "value": "alt@example.org", "type": "home" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type eq \"work\" and value co \"@example.com\"]" }
				]
			}`,
			want: `{
				"userType": "Employee",
				"emails": [
					{ "value": "alt@example.org", "type": "home" }
				]
			}`,
		},
		{
			name: "Filter_EmailsOrIMsXMPP",
			object: `{
				"emails": [
					{ "value": "bjensen@example.com", "type": "work" },
					{ "value": "alt@example.org", "type": "home" }
				],
				"ims": [
					{ "type": "xmpp", "value": "user@foo.com" }
				]
			}`,
			patch: `{
				"Operations": [
					{ "op": "remove", "path": "emails[type eq \"work\" and value co \"@example.com\"]" }
				]
			}`,
			want: `{
				"emails": [
					{ "value": "alt@example.org", "type": "home" }
				],
				"ims": [
					{ "type": "xmpp", "value": "user@foo.com" }
				]
			}`,
		},
		{
			name: "Patch_Members_DisplayName_ByValue",
			object: `{
				"members": [
					{
						"value": "2819c223-7f76-453a-919d-413861904646",
						"displayName": "Old Name"
					},
					{
						"value": "902c246b-6245-4190-8e05-00816be7344a",
						"displayName": "Someone Else"
					}
				]
			}`,
			patch: `{
				"Operations": [
					{
						"op": "replace",
						"path": "members[value eq \"2819c223-7f76-453a-919d-413861904646\"].displayName",
						"value": "Updated Name"
					}
				]
			}`,
			want: `{
				"members": [
					{
						"value": "2819c223-7f76-453a-919d-413861904646",
						"displayName": "Updated Name"
					},
					{
						"value": "902c246b-6245-4190-8e05-00816be7344a",
						"displayName": "Someone Else"
					}
				]
			}`,
		},
		{
			name: "patch scim entra object",
			object: `{
			  "active": true,
			  "addresses": [
				{
				  "type": "work",
				  "formatted": "YALSARLWETWH",
				  "streetAddress": "154 Keyshawn Underpass",
				  "locality": "WSUNPIXHZVVC",
				  "region": "WVOEILYHJFQQ",
				  "postalCode": "uz5 9vv",
				  "primary": true,
				  "country": "Cook Islands"
				}
			  ],
			  "displayName": "KOLCNFKVFDFR",
			  "emails": [
				{
				  "type": "work",
				  "value": "hal.kihn@botsford.uk",
				  "primary": true
				}
			  ],
			  "name": {
				"givenName": "Presley",
				"familyName": "Rubye"
			  },
			  "phoneNumbers": [
				{
				  "type": "work",
				  "value": "13-659-8102",
				  "primary": true
				},
				{
				  "type": "mobile",
				  "value": "13-659-8102"
				},
				{
				  "type": "fax",
				  "value": "13-659-8102"
				}
			  ],
			  "schemas": [
				"urn:ietf:params:scim:schemas:core:2.0:User",
				"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
			  ],
			  "userName": "christopher.harber@olson.name"
			}`,
			patch: `{
			  "schemas": [
				"urn:ietf:params:scim:api:messages:2.0:PatchOp"
			  ],
			  "Operations": [
				{
				  "op": "Add",
				  "path": "displayName",
				  "value": "Jon Lock"
				},
				{
				  "op": "Add",
				  "path": "emails[type eq \"work\"].value",
				  "value": "halk.ihn@botsford.uk"
				},
				{
				  "op": "Add",
				  "path": "addresses[type eq \"work\"].region",
				  "value": "02"
				},
				{
				  "op": "Add",
				  "path": "name.givenName",
				  "value": "Adam"
				}
			  ]
			}`,
			want: `{
			  "active": true,
			  "addresses": [
				{
				  "type": "work",
				  "formatted": "YALSARLWETWH",
				  "streetAddress": "154 Keyshawn Underpass",
				  "locality": "WSUNPIXHZVVC",
				  "region": "02",
				  "postalCode": "uz5 9vv",
				  "primary": true,
				  "country": "Cook Islands"
				}
			  ],
			  "displayName": "Jon Lock",
			  "emails": [
				{
				  "type": "work",
				  "value": "halk.ihn@botsford.uk",
				  "primary": true
				}
			  ],
			  "name": {
				"givenName": "Adam",
				"familyName": "Rubye"
			  },
			  "phoneNumbers": [
				{
				  "type": "work",
				  "value": "13-659-8102",
				  "primary": true
				},
				{
				  "type": "mobile",
				  "value": "13-659-8102"
				},
				{
				  "type": "fax",
				  "value": "13-659-8102"
				}
			  ],
			  "schemas": [
				"urn:ietf:params:scim:schemas:core:2.0:User",
				"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
			  ],
			  "userName": "christopher.harber@olson.name"
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := Apply([]byte(tc.object), []byte(tc.patch))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var got, want map[string]any
			err = json.Unmarshal(result, &got)
			require.NoError(t, err)
			err = json.Unmarshal([]byte(tc.want), &want)
			require.NoError(t, err)

			require.EqualValues(t, want, got)
		})
	}
}

func BenchmarkApply(b *testing.B) {
	var input = []byte(`{
		"name": {
			"givenName": "Barbara",
			"familyName": "Jensen"
		},
		"emails": [
			{ "value": "bjensen@example.com", "type": "work", "primary": true },
			{ "value": "babs@home.com", "type": "home" }
		]
	}`)

	var patchData = []byte(`{
		"Operations": [
			{ "op": "replace", "path": "name.familyName", "value": "O'Malley" },
			{ "op": "add", "path": "emails", "value": [
				{ "value": "new@example.com", "type": "other" }
			] },
			{ "op": "remove", "path": "emails[type eq \"home\"]" }
		]
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Apply(input, patchData)
		if err != nil {
			b.Fatalf("Apply failed: %v", err)
		}
	}
}

func FuzzApply(f *testing.F) {
	f.Add([]byte(`{"userName":"bjensen","emails":[{"value":"bjensen@example.com","type":"work"}]}`),
		[]byte(`{
			"schemas": ["urn:ietf:params:scim:api:messages:2.0:PatchOp"],
			"Operations": [
				{ "op": "replace", "path": "userName", "value": "newname" }
			]
		}`),
	)

	f.Fuzz(func(t *testing.T, target, patch []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic during fuzzing with input: %s\npatch: %s\npanic: %v", string(target), string(patch), r)
			}
		}()
		_, _ = Apply(target, patch)
	})
}

func FuzzParsePath(f *testing.F) {
	f.Add("userName")
	f.Add("name.familyName")
	f.Add("emails[type eq \"work\"]")
	f.Add("emails[type eq \"work\"].value")
	f.Add("emails[not primary pr]")
	f.Add("emails[type eq \"work\" and value co \"example\"]")
	f.Add("invalid[broken")

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic on parsePath(%q): %v", input, r)
			}
		}()
		_, _, _, _ = parsePath(input)
	})
}
