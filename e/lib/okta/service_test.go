package okta

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestBecomeLeader(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClockAt(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	ap := newTestAccessPoint(t, clock)

	const numServices = 10
	services := make([]*Service, numServices)

	// Start all of the services.
	ctx := context.Background()
	for i := 0; i < numServices; i++ {
		services[i], _, _ = newTestService(t, ap)
		services[i].hostID = strconv.Itoa(i)
		services[i].clock = clock
		require.NoError(t, services[i].Start(ctx))
	}

	orgURL := services[0].orgURLBase64

	var holderID string
	// Each service should grab the semaphore as the other services are stopped.
	for serviceCount := 0; serviceCount < numServices-1; serviceCount++ {
		// One semaphore lease should be retrieved.
		var semaphores []types.Semaphore
		require.Eventually(t, func() bool {
			var err error
			semaphores, err = ap.GetSemaphores(ctx, types.SemaphoreFilter{
				SemaphoreKind: semaphoreKind,
				SemaphoreName: orgURL,
			})
			if err != nil {
				return false
			}
			// the old holder ID shouldn't be the lease holder, it should have moved to a new service
			return len(semaphores) == 1 && len(semaphores[0].LeaseRefs()) == 1 && semaphores[0].LeaseRefs()[0].Holder != holderID
		}, 5*time.Second, 250*time.Millisecond)

		// Update the holder ID.
		holderID = semaphores[0].LeaseRefs()[0].Holder

		// Make sure the service denoted as the semaphore holder is the current leader, otherwise it shouldn't be.
		var leaderIndex int
		require.Eventually(t, func() bool {
			leaderIndex = slices.IndexFunc(services, func(s *Service) bool {
				return s.IsLeader()
			})
			return leaderIndex != -1 && strconv.Itoa(leaderIndex) == holderID
		}, 10*time.Second, 250*time.Millisecond, "leader host ID (%d) does not match holder ID (%s)", leaderIndex, holderID)

		require.NoError(t, services[leaderIndex].Shutdown())
		require.NoError(t, services[leaderIndex].Close(ctx))

		// Advance so that another service grabs the semaphore.
		clock.Advance(semaphoreRenewal * 100)
	}
}

func requireNotFound(t require.TestingT, err error, _ ...interface{}) {
	require.True(t, trace.IsNotFound(err), "Expected NotFound, got %s", err)
}

func TestSelectSelectCredentials(t *testing.T) {
	const ncreds = 4
	unlabeled := make([]types.PluginStaticCredentials, ncreds)
	for i := 0; i < ncreds; i++ {
		unlabeled[i] = &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeOkta,
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "some token",
				},
			},
		}
	}

	scimCred := make([]types.PluginStaticCredentials, ncreds)
	for i := 0; i < ncreds; i++ {
		scimCred[i] = &types.PluginStaticCredentialsV1{
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Name: types.PluginTypeOkta,
					Labels: map[string]string{
						CredPurposeLabel: CredPurposeSCIMToken,
					},
				},
			},
			Spec: &types.PluginStaticCredentialsSpecV1{
				Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
					APIToken: "some token",
				},
			},
		}
	}

	oktaToken := &types.PluginStaticCredentialsV1{
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Name: types.PluginTypeOkta,
				Labels: map[string]string{
					CredPurposeLabel: CredPurposeOktaAuth,
				},
			},
		},
		Spec: &types.PluginStaticCredentialsSpecV1{
			Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
				APIToken: "some token",
			},
		},
	}

	type testCase struct {
		name         string
		input        []types.PluginStaticCredentials
		expectedErr  require.ErrorAssertionFunc
		expectedCred types.PluginStaticCredentials
	}

	apiTokenTestCases := []testCase{
		{
			name:        "empty",
			expectedErr: requireNotFound,
		},
		{
			name:         "single, no purpose",
			input:        []types.PluginStaticCredentials{unlabeled[0]},
			expectedErr:  require.NoError,
			expectedCred: unlabeled[0],
		},
		{
			name:         "multiple, no labeled, picks first",
			input:        []types.PluginStaticCredentials{unlabeled[3], unlabeled[1]},
			expectedErr:  require.NoError,
			expectedCred: unlabeled[3],
		},
		{
			name:        "multiple, all scim, fails",
			input:       scimCred,
			expectedErr: requireNotFound,
		},
		{
			name: "multiple, one nopurpose, picks unlabeled",
			input: []types.PluginStaticCredentials{
				scimCred[3], scimCred[0], unlabeled[1], scimCred[2], unlabeled[0],
			},
			expectedErr:  require.NoError,
			expectedCred: unlabeled[1],
		}, {
			name: "multiple, one labeled oktatoken, picks labeled",
			input: []types.PluginStaticCredentials{
				scimCred[3], scimCred[0], oktaToken, scimCred[2], unlabeled[0],
			},
			expectedErr:  require.NoError,
			expectedCred: oktaToken,
		},
	}

	scimTokenTestCases := []testCase{
		{
			name:        "empty",
			expectedErr: requireNotFound,
		}, {
			name:        "single, no purpose",
			input:       []types.PluginStaticCredentials{unlabeled[0]},
			expectedErr: requireNotFound,
		}, {
			name: "mixed API and no purpose",
			input: []types.PluginStaticCredentials{
				unlabeled[1], oktaToken, unlabeled[0],
			},
			expectedErr: requireNotFound,
		}, {
			name:         "single SCIM token",
			input:        []types.PluginStaticCredentials{scimCred[2]},
			expectedErr:  require.NoError,
			expectedCred: scimCred[2],
		}, {
			name: "mixed tokens",
			input: []types.PluginStaticCredentials{
				unlabeled[2], oktaToken, scimCred[1], unlabeled[1],
			},
			expectedErr:  require.NoError,
			expectedCred: scimCred[1],
		}, {
			name: "multiple tokens, picks first",
			input: []types.PluginStaticCredentials{
				scimCred[3], scimCred[0], scimCred[1], scimCred[2],
			},
			expectedErr:  require.NoError,
			expectedCred: scimCred[3],
		},
	}

	t.Run("APIToken", func(t *testing.T) {
		for _, test := range apiTokenTestCases {
			t.Run(test.name, func(t *testing.T) {
				cred, err := SelectAPIToken(test.input)
				test.expectedErr(t, err)
				if test.expectedCred == nil {
					require.Nil(t, cred)
				} else {
					require.Same(t, test.expectedCred, cred)
				}
			})
		}
	})

	t.Run("SCIMToken", func(t *testing.T) {
		for _, test := range scimTokenTestCases {
			t.Run(test.name, func(t *testing.T) {
				cred, err := SelectSCIMToken(test.input)
				test.expectedErr(t, err)
				if test.expectedCred == nil {
					require.Nil(t, cred)
				} else {
					require.Same(t, test.expectedCred, cred)
				}
			})
		}
	})
}
