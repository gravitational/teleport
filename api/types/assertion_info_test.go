/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"encoding/xml"
	"testing"
	"time"

	saml2 "github.com/russellhaering/gosaml2"
	samltypes "github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/require"
)

func TestAssertionInfo_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		src  AssertionInfo
	}{
		{name: "empty", src: AssertionInfo{}},
		{name: "full", src: (AssertionInfo)(saml2.AssertionInfo{
			NameID: "zz",
			Values: map[string]samltypes.Attribute{
				"foo": {
					XMLName: xml.Name{
						Space: "ddd",
						Local: "aaa",
					},
					FriendlyName: "aaa",
					Name:         "aaa",
					NameFormat:   "",
					Values:       nil,
				},
			},
			WarningInfo: &saml2.WarningInfo{
				OneTimeUse: true,
				ProxyRestriction: &saml2.ProxyRestriction{
					Count:    1,
					Audience: []string{"foo"},
				},
				NotInAudience: true,
				InvalidTime:   true,
			},
			SessionIndex:        "aaa",
			AuthnInstant:        new(time.Time),
			SessionNotOnOrAfter: new(time.Time),
			Assertions: []samltypes.Assertion{
				{XMLName: xml.Name{Space: "urn:oasis:names:tc:SAML:2.0:assertion", Local: "AttributeValue"}},
			},
			ResponseSignatureValidated: true,
		})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := make([]byte, tt.src.Size())
			count, err := tt.src.MarshalTo(buf)
			require.NoError(t, err)
			require.Equal(t, tt.src.Size(), count)

			dst := &AssertionInfo{}
			err = dst.Unmarshal(buf)
			require.NoError(t, err)
			require.Equal(t, &tt.src, dst)
		})
	}
}
