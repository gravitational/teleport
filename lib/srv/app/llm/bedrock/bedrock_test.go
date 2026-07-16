// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package bedrock

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

func TestBuildURL(t *testing.T) {
	for name, tc := range map[string]struct {
		app           types.Application
		expectedError require.ErrorAssertionFunc
		expectedURL   require.ValueAssertionFunc
	}{
		"anthropic format": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{Region: "us-east-2"}),
			expectedError: require.NoError,
			expectedURL: func(tt require.TestingT, i1 any, i2 ...any) {
				u, _ := i1.(*url.URL)
				require.Equal(tt, "https", u.Scheme)
				require.Equal(tt, "bedrock-mantle.us-east-2.api.aws", u.Host)
				require.Equal(tt, "/anthropic/v1", u.Path)
			},
		},
		"openai format": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{Region: "eu-central-1"}),
			expectedError: require.NoError,
			expectedURL: func(tt require.TestingT, i1 any, i2 ...any) {
				u, _ := i1.(*url.URL)
				require.Equal(tt, "https", u.Scheme)
				require.Equal(tt, "bedrock-mantle.eu-central-1.api.aws", u.Host)
				require.Equal(tt, "/v1", u.Path)
			},
		},
		"default region": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatOpenAI,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{}),
			expectedError: require.NoError,
			expectedURL: func(tt require.TestingT, i1 any, i2 ...any) {
				u, _ := i1.(*url.URL)
				require.Equal(tt, "https", u.Scheme)
				require.Equal(tt, "bedrock-mantle."+defaultRegion+".api.aws", u.Host)
				require.Equal(tt, "/v1", u.Path)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			u, err := BuildURL(slog.Default(), tc.app)
			tc.expectedError(t, err)
			tc.expectedURL(t, u)
		})
	}
}

func TestSignRequest(t *testing.T) {
	app := newApp(t, &types.LLM{
		Format:   types.LLMFormatAnthropic,
		Provider: types.LLMProviderAWSBedrock,
	}, &types.AppAWS{Region: "us-east-2"})

	staticCredentials := func(gotRegion *string) awsconfig.Provider {
		return awsconfig.ProviderFunc(func(_ context.Context, region string, _ ...awsconfig.OptionsFn) (aws.Config, error) {
			*gotRegion = region
			return aws.Config{
				Credentials: credentials.NewStaticCredentialsProvider("AKID", "SECRET", "TOKEN"),
			}, nil
		})
	}

	newRequest := func() *http.Request {
		r, _ := http.NewRequest(http.MethodPost, "https://bedrock-mantle.us-east-2.api.aws/anthropic/v1/messages", nil)
		return r
	}

	for name, tc := range map[string]struct {
		app             types.Application
		request         func() *http.Request
		credentials     func(gotRegion *string) awsconfig.Provider
		expectedError   require.ErrorAssertionFunc
		expectedRequest require.ValueAssertionFunc
		expectedRegion  string
	}{
		"successful": {
			app:            app,
			request:        newRequest,
			credentials:    staticCredentials,
			expectedError:  require.NoError,
			expectedRegion: "us-east-2",
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Contains(tt, req.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
				require.Contains(tt, req.Header.Get("Authorization"), mantleServiceName)
				require.NotEmpty(tt, req.Header.Get("X-Amz-Date"))
				require.Equal(tt, "TOKEN", req.Header.Get("X-Amz-Security-Token"))
			},
		},
		"successful with default region": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{Region: ""}),
			request:        newRequest,
			credentials:    staticCredentials,
			expectedError:  require.NoError,
			expectedRegion: defaultRegion,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Contains(tt, req.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
				require.Contains(tt, req.Header.Get("Authorization"), mantleServiceName)
				require.NotEmpty(tt, req.Header.Get("X-Amz-Date"))
				require.Equal(tt, "TOKEN", req.Header.Get("X-Amz-Security-Token"))
			},
		},
		"successful with integration": {
			app: newApp(t, &types.LLM{
				Format:   types.LLMFormatAnthropic,
				Provider: types.LLMProviderAWSBedrock,
			}, &types.AppAWS{Region: "us-east-2"}, withIntegration("my-integration")),
			request:        newRequest,
			credentials:    staticCredentials,
			expectedError:  require.NoError,
			expectedRegion: "us-east-2",
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Contains(tt, req.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
				require.NotEmpty(tt, req.Header.Get("X-Amz-Date"))
			},
		},
		"get config failure": {
			app:     app,
			request: newRequest,
			credentials: func(*string) awsconfig.Provider {
				return awsconfig.ProviderFunc(func(context.Context, string, ...awsconfig.OptionsFn) (aws.Config, error) {
					return aws.Config{}, errors.New("get config failed")
				})
			},
			expectedError: require.Error,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Empty(tt, req.Header.Get("Authorization"))
			},
		},
		"retrieve credentials failure": {
			app:     app,
			request: newRequest,
			credentials: func(*string) awsconfig.Provider {
				return awsconfig.ProviderFunc(func(context.Context, string, ...awsconfig.OptionsFn) (aws.Config, error) {
					return aws.Config{Credentials: failingCredentialsProvider{}}, nil
				})
			},
			expectedError: require.Error,
			expectedRequest: func(tt require.TestingT, i1 any, i2 ...any) {
				req, _ := i1.(*http.Request)
				require.Empty(tt, req.Header.Get("Authorization"))
			},
		},
		"missing app": {
			app:             nil,
			request:         newRequest,
			credentials:     staticCredentials,
			expectedError:   require.Error,
			expectedRequest: require.NotNil,
		},
		"missing credentials": {
			app:             app,
			request:         newRequest,
			credentials:     func(*string) awsconfig.Provider { return nil },
			expectedError:   require.Error,
			expectedRequest: require.NotNil,
		},
		"missing request": {
			app:             app,
			request:         func() *http.Request { return nil },
			credentials:     staticCredentials,
			expectedError:   require.Error,
			expectedRequest: require.Nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var gotRegion string
			opts := SignRequestOptions{
				App:         tc.app,
				Credentials: tc.credentials(&gotRegion),
				Request:     tc.request(),
				RequestBody: []byte(`{"model":"claude-sonnet-4-20250514"}`),
			}

			err := SignRequest(t.Context(), opts)
			tc.expectedError(t, err)
			tc.expectedRequest(t, opts.Request)
			require.Equal(t, tc.expectedRegion, gotRegion)
		})
	}
}

// failingCredentialsProvider is an [aws.CredentialsProvider] that always fails
// to retrieve credentials.
type failingCredentialsProvider struct{}

func (failingCredentialsProvider) Retrieve(context.Context) (aws.Credentials, error) {
	return aws.Credentials{}, errors.New("retrieve failed")
}

type appOption func(*types.AppSpecV3)

func withIntegration(name string) appOption {
	return func(spec *types.AppSpecV3) {
		spec.Integration = name
	}
}

func newApp(t *testing.T, llm *types.LLM, appAWS *types.AppAWS, opts ...appOption) types.Application {
	t.Helper()
	spec := types.AppSpecV3{
		LLM: llm,
		AWS: appAWS,
	}
	for _, opt := range opts {
		opt(&spec)
	}
	app, err := types.NewAppV3(types.Metadata{Name: "llm-app"}, spec)
	require.NoError(t, err)
	return app
}
