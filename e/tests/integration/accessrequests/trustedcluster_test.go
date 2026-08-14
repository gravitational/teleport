package accessrequests

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	eui "github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/slices"
)

type trustedClusterOptions struct {
	labels  map[string]string
	roleMap types.RoleMap
}

type trustedClusterOption func(*trustedClusterOptions)

func withLabel(key, value string) trustedClusterOption {
	return func(opts *trustedClusterOptions) {
		if opts.labels == nil {
			opts.labels = map[string]string{}
		}
		opts.labels[key] = value
	}
}

func withRoleMapping(rootRole string, leafRoles ...string) trustedClusterOption {
	if len(leafRoles) == 0 {
		panic("must supply at least one leaf role")
	}

	return func(opts *trustedClusterOptions) {
		opts.roleMap = append(opts.roleMap, types.RoleMapping{Remote: rootRole, Local: leafRoles})
	}
}

func mustCreateTrustedCluster(ctx context.Context, t *testing.T, rootCluster, leafCluster *common.SUT, options ...trustedClusterOption) {
	var tcOptions trustedClusterOptions
	for _, optionFn := range options {
		optionFn(&tcOptions)
	}

	rootAuth := rootCluster.Teleport.Process.GetAuthServer()
	token, err := types.NewProvisionToken(uuid.NewString(), []types.SystemRole{types.RoleTrustedCluster}, time.Time{})
	require.NoError(t, err)
	require.NoError(t, rootAuth.UpsertToken(ctx, token))

	trustedCluster := rootCluster.Teleport.AsTrustedCluster(
		token.GetName(),
		tcOptions.roleMap,
	)

	leafAuth := leafCluster.Teleport.Process.GetAuthServer()
	_, err = leafAuth.CreateTrustedCluster(ctx, trustedCluster)
	require.NoError(t, err, "Creating trusted cluster %q", trustedCluster.GetName())

	leafClusterName := leafCluster.Teleport.Secrets.SiteName

	var rc types.RemoteCluster
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			rc, err = rootAuth.GetRemoteCluster(ctx, leafClusterName)
			assert.NoError(c, err)
		},
		5*time.Second, 250*time.Millisecond,
		"Looking up remote cluster")
	require.NotNil(t, rc)

	if len(tcOptions.labels) == 0 {
		return
	}

	rcMeta := rc.GetMetadata()
	if rcMeta.Labels == nil {
		rcMeta.Labels = make(map[string]string)
	}
	maps.Copy(rcMeta.Labels, tcOptions.labels)
	rc.SetMetadata(rcMeta)

	_, err = rootAuth.UpdateRemoteCluster(ctx, rc)
	require.NoError(t, err)
}

type responseBodyAssertion func(require.TestingT, []byte)

type genericResource struct {
	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
	Kind string `json:"kind"`
	// SubKind is a node subkind such as OpenSSH
	SubKind string `json:"subKind"`
	// Name is this server name
	Name string `json:"id"`
	// ClusterName is this server cluster name
	ClusterName string `json:"siteId"`
}

func toResourceID(r genericResource) eui.ResourceID {
	return eui.ResourceID{
		ClusterName: r.ClusterName,
		Kind:        r.Kind,
		Name:        r.Name,
	}
}

func requireResources(expectedResourceIDs ...eui.ResourceID) responseBodyAssertion {
	return func(t require.TestingT, body []byte) {
		var resources resourceList[genericResource]
		require.NoError(t, json.Unmarshal(body, &resources))
		resourceIDs := slices.Map(resources.Items, toResourceID)
		require.ElementsMatch(t, expectedResourceIDs, resourceIDs)
	}
}

func requireRoleNames(expected ...string) responseBodyAssertion {
	return func(t require.TestingT, body []byte) {
		var roles []string
		require.NoError(t, json.Unmarshal(body, &roles))
		require.ElementsMatch(t, expected, roles)
	}
}

func TestRemoteResourceAccessRequests(t *testing.T) {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.SetDefault(
		slog.New(logutils.NewSlogTextHandler(
			os.Stdout, logutils.SlogTextHandlerConfig{Level: slog.LevelDebug})))
	ctx := t.Context()

	// Set up a root cluster for the tests to interact with
	rootCluster := common.InitSUT(t,
		common.WithClusterName("root"),
		common.WithInsecure(),
		common.WithLogger(slog.With("cluster", "root")),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "leaf-cluster-access", common.WithClusterLabel(types.Allow, "department", "ai-farm")),

		// Create requester+access roles that will be mapped to symmetric
		// requester & access roles in the leaf cluster
		common.WithRole(t, "all-ai-access"),
		common.WithRole(t, "all-ai-requester", common.WithSearchAs(types.Allow, "all-ai-access")),

		// Create some roles that will not be mapped to symmetric roles on the
		// leaf cluster; only the access roles will be mapped.
		common.WithRole(t, "stable-ai-access"),
		common.WithRole(t, "stable-ai-requester", common.WithSearchAs(types.Allow, "stable-ai-access")),
		common.WithRole(t, "unstable-ai-access"),
		common.WithRole(t, "unstable-ai-requester", common.WithSearchAs(types.Allow, "unstable-ai-access")),

		// Create a role that will map to a leaf-cluster role with search_as access
		// to "unstable AI" resources, but has no corresponding `search_as` role on
		// the root cluster.
		common.WithRole(t, "unstable-ai-browser"),

		// Add some root-cluster users with various roles
		common.WithUser(t, "root-admin", "editor", "leaf-cluster-access"),
	)
	rootAuth := rootCluster.GetClusterClientForUser(t, "root-admin")

	// Set up a leaf cluster for the root cluster to delegate requests to
	leafCluster := common.InitSUT(t,
		common.WithClusterName("leaf"),
		common.WithLogger(slog.With("cluster", "leaf")),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithRole(t, "leaf-access-stable-ai", common.WithRoleNodeLabel(types.Allow, "stable", "yes")),
		common.WithRole(t, "leaf-access-unstable-ai", common.WithRoleNodeLabel(types.Allow, "stable", "no")),
		common.WithRole(t, "leaf-access-all-ai", common.WithRoleNodeLabel(types.Allow, "stable", "*")),
		common.WithRole(t, "leaf-request-all-ai", common.WithSearchAs(types.Allow, "leaf-access-all-ai")),
		common.WithRole(t, "leaf-browse-unstable-ai", common.WithSearchAs(types.Allow, "leaf-access-unstable-ai")),
		common.WithRole(t, "visitor"),
		common.WithUser(t, "leaf-admin", "editor"),
		common.WithInsecure(),
	)
	leafAuth := leafCluster.GetClusterClientForUser(t, "leaf-admin")

	// Create some leaf-cluster resources for the user to access
	mustCreateNode(ctx, t, leafAuth.AuthClient, "deep-thought", "magerathera.example.com",
		common.WithNodeLabel("stable", "yes"))
	mustCreateNode(ctx, t, leafAuth.AuthClient, "hex", "hex.unuseen-u.example.com",
		common.WithNodeLabel("stable", "yes"))
	mustCreateNode(ctx, t, leafAuth.AuthClient, "hal-9000", "uss-discovery.example.com",
		common.WithNodeLabel("stable", "no"))
	mustCreateNode(ctx, t, leafAuth.AuthClient, "shodan", "citadel-station.example.com",
		common.WithNodeLabel("stable", "no"))

	// ...and another resource that isn't allowed by ANY role, as a canary; if this
	// ever shows up in resource list then something is very wrong.
	mustCreateNode(ctx, t, leafAuth.AuthClient, "not-an-ai", "root.example.com")

	// Create trusted cluster relationship between the two
	mustCreateTrustedCluster(ctx, t, rootCluster, leafCluster,
		withLabel("department", "ai-farm"),
		withRoleMapping("leaf-cluster-access", "visitor"),

		// mapping for symmetric requester/access role pairs
		withRoleMapping("all-ai-access", "leaf-access-all-ai"),
		withRoleMapping("all-ai-requester", "leaf-request-all-ai"),

		// mapping for access-only roles
		withRoleMapping("stable-ai-access", "leaf-access-stable-ai"),
		withRoleMapping("unstable-ai-access", "leaf-access-unstable-ai"),

		// mapping for a role that only has a search_as role on the leaf-cluster end
		withRoleMapping("unstable-ai-browser", "leaf-browse-unstable-ai"),
	)

	stableAIs := []eui.ResourceID{
		{Kind: types.KindNode, Name: "hex", ClusterName: "leaf"},
		{Kind: types.KindNode, Name: "deep-thought", ClusterName: "leaf"},
	}

	unstableAIs := []eui.ResourceID{
		{Kind: types.KindNode, Name: "hal-9000", ClusterName: "leaf"},
		{Kind: types.KindNode, Name: "shodan", ClusterName: "leaf"},
	}

	allAIs := append(stableAIs, unstableAIs...)

	// Wait for the trusted cluster to come up
	require.EventuallyWithT(t,
		func(c *assert.CollectT) {
			leafClient, err := rootAuth.ConnectToCluster(ctx, "leaf")
			if !assert.NoError(c, err, "connecting to remote cluster") {
				return
			}
			defer leafClient.Close()

			_, err = leafClient.Ping(ctx)
			if !assert.NoError(c, err, "pinging remote cluster") {
				return
			}
		},
		5*time.Second, 250*time.Millisecond, "Waiting for trusted cluster to come up")

	// Now, the actual tests:

	t.Run("list remote resources", func(t *testing.T) {
		testCases := []struct {
			name               string
			userRoles          []string
			expectedHTTPStatus int
			expectedBody       responseBodyAssertion
		}{
			{
				name:               "No cluster access",
				userRoles:          []string{"stable-ai-requester"},
				expectedHTTPStatus: http.StatusNotFound,
			},
			{
				name:               "Symmetric roles mapping access",
				userRoles:          []string{"leaf-cluster-access", "all-ai-requester"},
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireResources(allAIs...),
			},
			{
				// Asserts that a request will return a set of resources based the
				// mapped leaf-cluster user's roleset, regardless of what the role-cluster
				// user's search_as roleset maps to.
				name:               "Asymmetric role mapping access",
				userRoles:          []string{"leaf-cluster-access", "stable-ai-requester", "unstable-ai-requester"},
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireResources(),
			},
			{
				// A request from a user with an empty root-cluster search_as roleset
				// should return resources that the leaf-cluster user can search for
				name:               "No root-cluster search_as roles",
				userRoles:          []string{"leaf-cluster-access", "editor", "unstable-ai-browser"},
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireResources(unstableAIs...),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				username, err := utils.CryptoRandomHex(8)
				require.NoError(t, err)

				common.MustCreateUserWithCleanup(t, rootCluster, username, testCase.userRoles...)
				client := rootCluster.CreateWebClientForUser(t, username)

				query := url.Values{
					"includedResourceMode": {"all"},
					"kinds":                {types.KindNode},
				}
				endpoint := "sites/leaf/resources?" + query.Encode()

				status, response := client.DoWebAPIRequest(t, http.MethodGet, endpoint, nil)
				require.Equal(t, testCase.expectedHTTPStatus, status)
				if testCase.expectedBody != nil {
					testCase.expectedBody(t, response)
				}
			})
		}
	})

	t.Run("prune remote roles", func(t *testing.T) {
		testCases := []struct {
			name               string
			userRoles          []string
			resources          []eui.ResourceID
			expectedHTTPStatus int
			expectedBody       responseBodyAssertion
		}{
			{
				name:               "Empty request",
				userRoles:          []string{"leaf-cluster-access", "stable-ai-requester"},
				resources:          []eui.ResourceID{},
				expectedHTTPStatus: http.StatusOK,
			},
			{
				name:               "No remote cluster access",
				userRoles:          []string{"stable-ai-requester"},
				resources:          allAIs,
				expectedHTTPStatus: http.StatusNotFound,
			},
			{
				name:               "Single resource",
				userRoles:          []string{"leaf-cluster-access", "stable-ai-requester", "unstable-ai-requester", "all-ai-requester"},
				resources:          stableAIs[1:],
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireRoleNames("stable-ai-access", "all-ai-access"),
			},
			{
				name:               "Multiple resources requiring multiple roles",
				userRoles:          []string{"leaf-cluster-access", "stable-ai-requester", "unstable-ai-requester"},
				resources:          allAIs,
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireRoleNames("stable-ai-access", "unstable-ai-access"),
			},
			{
				// A request from a user with an empty root-cluster search_as roleset
				// should return an empty list
				name:               "No root-cluster search_as roles",
				userRoles:          []string{"leaf-cluster-access", "editor"},
				resources:          allAIs,
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireRoleNames(),
			},
			{
				// A request from a user with a root-cluster search_as roleset that
				// doesn't map to any leaf-cluster roles should return an empty list
				name:               "No leaf-cluster search_as roles",
				userRoles:          []string{"leaf-cluster-access", "unstable-ai-browser", "requester"},
				resources:          unstableAIs,
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireRoleNames(),
			},
			{
				// A request for resources that cannot be accessed via the supplied roles
				name:               "Unreachable resources",
				userRoles:          []string{"leaf-cluster-access", "unstable-ai-browser", "requester"},
				resources:          allAIs,
				expectedHTTPStatus: http.StatusOK,
				expectedBody:       requireRoleNames(),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				username, err := utils.CryptoRandomHex(8)
				require.NoError(t, err)

				common.MustCreateUserWithCleanup(t, rootCluster, username, testCase.userRoles...)
				client := rootCluster.CreateWebClientForUser(t, username)

				resourceIDs, err := json.Marshal(testCase.resources)
				require.NoError(t, err)

				targetURL, err := url.Parse(client.Endpoint("v1", "enterprise", "resourcerequestroles"))
				require.NoError(t, err)
				query := url.Values{
					"resourceIds": []string{string(resourceIDs)},
				}
				targetURL.RawQuery = query.Encode()

				status, response, err := client.DoRequest(http.MethodGet, targetURL.String(), nil)
				require.NoError(t, err)
				require.Equal(t, testCase.expectedHTTPStatus, status)
				if testCase.expectedBody != nil {
					testCase.expectedBody(t, response)
				}
			})
		}
	})
}
