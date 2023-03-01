/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package saml

import (
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestSAMLSessionToWebSession(t *testing.T) {
	clock := clockwork.NewFakeClock()

	session := &saml.Session{
		ID:         "id",
		CreateTime: clock.Now(),
		ExpireTime: clock.Now().Add(time.Hour),
		Index:      "index",

		NameID:       "name-id",
		NameIDFormat: "name-id-format",
		SubjectID:    "subject-id",

		Groups:                []string{"group1", "group2"},
		UserName:              "user-name",
		UserEmail:             "user-email",
		UserCommonName:        "user-common-name",
		UserSurname:           "user-surname",
		UserGivenName:         "user-given-name",
		UserScopedAffiliation: "user-scoped-affiliation",

		CustomAttributes: []saml.Attribute{
			{
				FriendlyName: "friendly-name1",
				Name:         "name1",
				NameFormat:   "name-format1",
				Values: []saml.AttributeValue{
					{
						Type:  "type1-1",
						Value: "value1-1",
						NameID: &saml.NameID{
							NameQualifier:   "name-qualifier",
							SPNameQualifier: "sp-name-qualifier",
							Format:          "format",
							SPProvidedID:    "sp-provided-id",
							Value:           "value",
						},
					},
				},
			},
			{
				FriendlyName: "friendly-name2",
				Name:         "name2",
				NameFormat:   "name-format2",
				Values: []saml.AttributeValue{
					{
						Type:  "type2-1",
						Value: "value2-1",
					},
					{
						Type:  "type2-2",
						Value: "value2-2",
					},
				},
			},
		},
	}

	webSession, err := samlSessionToWebSession(session)
	require.NoError(t, err)

	expectedWebSession, err := types.NewWebSession("id", types.KindSAMLIdPSession, types.WebSessionSpecV2{
		User:    "user-name",
		Expires: clock.Now().Add(time.Hour),
		SAMLSession: &types.SAMLSessionData{
			ID: "id",

			CreateTime: clock.Now(),
			ExpireTime: clock.Now().Add(time.Hour),
			Index:      "index",

			NameID:       "name-id",
			NameIDFormat: "name-id-format",
			SubjectID:    "subject-id",

			Groups:                []string{"group1", "group2"},
			UserName:              "user-name",
			UserEmail:             "user-email",
			UserCommonName:        "user-common-name",
			UserSurname:           "user-surname",
			UserGivenName:         "user-given-name",
			UserScopedAffiliation: "user-scoped-affiliation",

			CustomAttributes: []*types.SAMLAttribute{
				{
					FriendlyName: "friendly-name1",
					Name:         "name1",
					NameFormat:   "name-format1",
					Values: []*types.SAMLAttributeValue{
						{
							Type:  "type1-1",
							Value: "value1-1",
							NameID: &types.SAMLNameID{
								NameQualifier:   "name-qualifier",
								SPNameQualifier: "sp-name-qualifier",
								Format:          "format",
								SPProvidedID:    "sp-provided-id",
								Value:           "value",
							},
						},
					},
				},
				{
					FriendlyName: "friendly-name2",
					Name:         "name2",
					NameFormat:   "name-format2",
					Values: []*types.SAMLAttributeValue{
						{
							Type:  "type2-1",
							Value: "value2-1",
						},
						{
							Type:  "type2-2",
							Value: "value2-2",
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expectedWebSession, webSession))
}

func TestWebSessionToSAMLSession(t *testing.T) {
	clock := clockwork.NewFakeClock()

	webSession, err := types.NewWebSession("id", types.KindSAMLIdPSession, types.WebSessionSpecV2{
		User:    "user-name",
		Expires: clock.Now().Add(time.Hour),
		SAMLSession: &types.SAMLSessionData{
			ID: "id",

			CreateTime: clock.Now(),
			ExpireTime: clock.Now().Add(time.Hour),
			Index:      "index",

			NameID:       "name-id",
			NameIDFormat: "name-id-format",
			SubjectID:    "subject-id",

			Groups:                []string{"group1", "group2"},
			UserName:              "user-name",
			UserEmail:             "user-email",
			UserCommonName:        "user-common-name",
			UserSurname:           "user-surname",
			UserGivenName:         "user-given-name",
			UserScopedAffiliation: "user-scoped-affiliation",

			CustomAttributes: []*types.SAMLAttribute{
				{
					FriendlyName: "friendly-name1",
					Name:         "name1",
					NameFormat:   "name-format1",
					Values: []*types.SAMLAttributeValue{
						{
							Type:  "type1-1",
							Value: "value1-1",
							NameID: &types.SAMLNameID{
								NameQualifier:   "name-qualifier",
								SPNameQualifier: "sp-name-qualifier",
								Format:          "format",
								SPProvidedID:    "sp-provided-id",
								Value:           "value",
							},
						},
					},
				},
				{
					FriendlyName: "friendly-name2",
					Name:         "name2",
					NameFormat:   "name-format2",
					Values: []*types.SAMLAttributeValue{
						{
							Type:  "type2-1",
							Value: "value2-1",
						},
						{
							Type:  "type2-2",
							Value: "value2-2",
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	session, err := webSessionToSAMLSession(webSession)
	require.NoError(t, err)

	expectedSession := &saml.Session{
		ID:         "id",
		CreateTime: clock.Now(),
		ExpireTime: clock.Now().Add(time.Hour),
		Index:      "index",

		NameID:       "name-id",
		NameIDFormat: "name-id-format",
		SubjectID:    "subject-id",

		Groups:                []string{"group1", "group2"},
		UserName:              "user-name",
		UserEmail:             "user-email",
		UserCommonName:        "user-common-name",
		UserSurname:           "user-surname",
		UserGivenName:         "user-given-name",
		UserScopedAffiliation: "user-scoped-affiliation",

		CustomAttributes: []saml.Attribute{
			{
				FriendlyName: "friendly-name1",
				Name:         "name1",
				NameFormat:   "name-format1",
				Values: []saml.AttributeValue{
					{
						Type:  "type1-1",
						Value: "value1-1",
						NameID: &saml.NameID{
							NameQualifier:   "name-qualifier",
							SPNameQualifier: "sp-name-qualifier",
							Format:          "format",
							SPProvidedID:    "sp-provided-id",
							Value:           "value",
						},
					},
				},
			},
			{
				FriendlyName: "friendly-name2",
				Name:         "name2",
				NameFormat:   "name-format2",
				Values: []saml.AttributeValue{
					{
						Type:  "type2-1",
						Value: "value2-1",
					},
					{
						Type:  "type2-2",
						Value: "value2-2",
					},
				},
			},
		},
	}

	require.Empty(t, cmp.Diff(expectedSession, session))
}
