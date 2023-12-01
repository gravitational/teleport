/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type fakeCredentialsGetter struct {
}

func (f *fakeCredentialsGetter) Get(ctx context.Context, request GetCredentialsRequest) (*credentials.Credentials, error) {
	return credentials.NewStaticCredentials(
		fmt.Sprintf("%s-%s-%s", request.SessionName, request.RoleARN, request.ExternalID),
		uuid.NewString(),
		uuid.NewString(),
	), nil
}

func TestCachedCredentialsGetter(t *testing.T) {
	t.Parallel()

	hostSession := session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.AnonymousCredentials,
		Region:      aws.String("us-west-2"),
	}))
	fakeClock := clockwork.NewFakeClock()

	getter, err := NewCachedCredentialsGetter(CachedCredentialsGetterConfig{
		Getter:   &fakeCredentialsGetter{},
		CacheTTL: time.Minute,
		Clock:    fakeClock,
	})
	require.NoError(t, err)

	cred1, err := getter.Get(context.Background(), GetCredentialsRequest{
		Provider:    hostSession,
		Expiry:      fakeClock.Now().Add(time.Hour),
		SessionName: "test-session",
		RoleARN:     "test-role",
	})
	require.NoError(t, err)
	checkCredentialsAccessKeyID(t, cred1, "test-session-test-role-")

	tests := []struct {
		name         string
		sessionName  string
		roleARN      string
		externalID   string
		advanceClock time.Duration
		compareCred1 require.ComparisonAssertionFunc
	}{
		{
			name:         "cached",
			sessionName:  "test-session",
			roleARN:      "test-role",
			compareCred1: require.Same,
		},
		{
			name:         "different session name",
			sessionName:  "test-session-2",
			roleARN:      "test-role",
			compareCred1: require.NotSame,
		},
		{
			name:         "different role ARN",
			sessionName:  "test-session",
			roleARN:      "test-role-2",
			compareCred1: require.NotSame,
		},
		{
			name:         "different external ID",
			sessionName:  "test-session",
			roleARN:      "test-role",
			externalID:   "test-id",
			compareCred1: require.NotSame,
		},
		{
			name:         "cache expired",
			sessionName:  "test-session",
			roleARN:      "test-role",
			advanceClock: time.Hour,
			compareCred1: require.NotSame,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClock.Advance(test.advanceClock)

			cred, err := getter.Get(context.Background(), GetCredentialsRequest{
				Provider:    hostSession,
				Expiry:      fakeClock.Now().Add(time.Hour),
				SessionName: test.sessionName,
				RoleARN:     test.roleARN,
				ExternalID:  test.externalID,
			})
			require.NoError(t, err)
			checkCredentialsAccessKeyID(t, cred, fmt.Sprintf("%s-%s-%s", test.sessionName, test.roleARN, test.externalID))
			test.compareCred1(t, cred1, cred)
		})
	}
}

func checkCredentialsAccessKeyID(t *testing.T, cred *credentials.Credentials, wantAccessKeyID string) {
	t.Helper()
	value, err := cred.Get()
	require.NoError(t, err)
	require.Equal(t, wantAccessKeyID, value.AccessKeyID)
}
