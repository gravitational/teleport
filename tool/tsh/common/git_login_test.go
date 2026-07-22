/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
)

func Test_getGitHubIdentity(t *testing.T) {
	cf := &CLIConf{
		Proxy:    "proxy",
		Username: "github-username",
		HomePath: t.TempDir(),
		Context:  context.Background(),
	}

	tests := []struct {
		name                 string
		initialProfileStatus *client.ProfileStatus
		options              []getGitHubIdentityOption
		wantError            bool
		wantIdentity         *client.GitHubIdentity
	}{
		{
			name: "GitHub identity already present",
			initialProfileStatus: &client.ProfileStatus{
				GitHubIdentity: &client.GitHubIdentity{
					Username: "github-username",
					UserID:   "1234567",
				},
			},
			wantIdentity: &client.GitHubIdentity{
				Username: "github-username",
				UserID:   "1234567",
			},
		},
		{
			name: "GitHub OAuth success",
			initialProfileStatus: &client.ProfileStatus{
				GitHubIdentity: nil,
			},
			options: []getGitHubIdentityOption{
				withOAuthFlowOverride(func(conf *CLIConf, org string) error {
					conf.profileStatusOverride = &client.ProfileStatus{
						GitHubIdentity: &client.GitHubIdentity{
							Username: "github-username",
							UserID:   "1234567",
						},
					}
					return nil
				}),
			},
			wantIdentity: &client.GitHubIdentity{
				Username: "github-username",
				UserID:   "1234567",
			},
		},
		{
			name: "GitHub OAuth failure",
			initialProfileStatus: &client.ProfileStatus{
				GitHubIdentity: nil,
			},
			options: []getGitHubIdentityOption{
				withOAuthFlowOverride(func(conf *CLIConf, org string) error {
					return trace.NotFound("%s not found", org)
				}),
			},
			wantError: true,
		},
		{
			name: "force GitHub OAuth",
			initialProfileStatus: &client.ProfileStatus{
				GitHubIdentity: &client.GitHubIdentity{
					Username: "username-github",
					UserID:   "7654321",
				},
			},
			options: []getGitHubIdentityOption{
				withForceOAuthFlow(true),
				withOAuthFlowOverride(func(conf *CLIConf, org string) error {
					conf.profileStatusOverride = &client.ProfileStatus{
						GitHubIdentity: &client.GitHubIdentity{
							Username: "github-username",
							UserID:   "1234567",
						},
					}
					return nil
				}),
			},
			wantIdentity: &client.GitHubIdentity{
				Username: "github-username",
				UserID:   "1234567",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cf.profileStatusOverride = test.initialProfileStatus

			identity, err := getGitHubIdentity(cf, "org", test.options...)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantIdentity, identity)
			}
		})
	}
}
