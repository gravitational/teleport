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

package pagerduty

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func newMinimalValidConfig() Config {
	return Config{
		Pagerduty: PagerdutyConfig{
			APIKey:    "some-api-key-string",
			UserEmail: "root@localhost",
		},
	}
}

func TestInvalidConfigFailsCheck(t *testing.T) {

	t.Run("API Key", func(t *testing.T) {
		uut := newMinimalValidConfig()
		uut.Pagerduty.APIKey = ""
		require.Error(t, uut.CheckAndSetDefaults())
	})

	t.Run("User Email", func(t *testing.T) {
		uut := newMinimalValidConfig()
		uut.Pagerduty.UserEmail = ""
		require.Error(t, uut.CheckAndSetDefaults())
	})
}

func TestConfigDefaults(t *testing.T) {
	uut := Config{
		Pagerduty: PagerdutyConfig{
			APIKey:    "some-api-key-string",
			UserEmail: "root@localhost",
		},
	}
	require.NoError(t, uut.CheckAndSetDefaults())
	require.Equal(t, APIEndpointDefaultURL, uut.Pagerduty.APIEndpoint)
	require.Equal(t, NotifyServiceDefaultAnnotation, uut.Pagerduty.RequestAnnotations.NotifyService)
	require.Equal(t, ServicesDefaultAnnotation, uut.Pagerduty.RequestAnnotations.Services)
	require.Equal(t, "stderr", uut.Log.Output)
	require.Equal(t, "info", uut.Log.Severity)
}
