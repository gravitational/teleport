/*
Copyright 2017 Gravitational, Inc.

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

package services

import (
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func TestParseFromMetadata(t *testing.T) {
	t.Parallel()

	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	oc, err := UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)
	err = ValidateSAMLConnector(oc)
	require.NoError(t, err)
	require.Equal(t, oc.GetIssuer(), "http://www.okta.com/exkafftca6RqPVgyZ0h7")
	require.Equal(t, oc.GetSSO(), "https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml")
	require.Equal(t, oc.GetAssertionConsumerService(), "https://localhost:3080/v1/webapi/saml/acs")
	require.Equal(t, oc.GetAudience(), "https://localhost:3080/v1/webapi/saml/acs")
	require.NotNil(t, oc.GetSigningKeyPair())
	require.Empty(t, cmp.Diff(oc.GetAttributes(), []string{"groups"}))
}

func TestCheckSAMLEntityDescriptor(t *testing.T) {
	t.Parallel()

	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	oc, err := UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)

	ed := oc.GetEntityDescriptor()
	certs, err := CheckSAMLEntityDescriptor(ed)
	require.NoError(t, err)
	require.Len(t, certs, 1)
}
