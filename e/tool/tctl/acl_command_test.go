package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/utils/keys"
	authe "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/scopes"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestACLList tests access lists CLI for displaying access lists.
func TestACLList(t *testing.T) {
	t.Parallel()
	client := setupACLSuite(t)

	mustCreateAccessList(t, client, "", "past-due", time.Now().UTC().Add(-24*time.Hour))
	mustCreateAccessList(t, client, "", "due", time.Now().UTC().Add(24*time.Hour))
	mustCreateAccessList(t, client, "", "not-yet-due", time.Now().UTC().Add(30*24*time.Hour))
	mustCreateAccessList(t, client, "/engineering", "past-due", time.Now().UTC().Add(-24*time.Hour))
	mustCreateAccessList(t, client, "/engineering", "due", time.Now().UTC().Add(24*time.Hour))
	mustCreateAccessList(t, client, "/engineering", "not-yet-due", time.Now().UTC().Add(30*24*time.Hour))

	// Fetch all lists.
	out, err := runACLCommand(t, client, []string{
		"ls", "--format", "json",
	})
	require.NoError(t, err)
	allLists := getAccessListNames(t, out)
	require.ElementsMatch(t, []string{
		"past-due", "due", "not-yet-due",
		"/engineering::past-due", "/engineering::due", "/engineering::not-yet-due",
	}, allLists)

	// Fetch only lists that are due a review.
	out, err = runACLCommand(t, client, []string{
		"ls", "--review-only", "--format", "json",
	})
	require.NoError(t, err)
	reviewOnlyLists := getAccessListNames(t, out)
	require.ElementsMatch(t, []string{
		"past-due", "due",
		"/engineering::past-due", "/engineering::due",
	}, reviewOnlyLists)
}

// TestACLReviews tests access lists CLI for managing access list reviews.
func TestACLReviews(t *testing.T) {
	t.Parallel()
	client := setupACLSuite(t)

	for _, tc := range []struct {
		name  string
		scope string
	}{
		{name: "unscoped"},
		{name: "scoped", scope: "/engineering"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const name = "test"
			accessListName := scopes.QualifiedName{Scope: tc.scope, Name: name}.String()
			removedMembers := "alice,bob"
			expectedMembers := []string{"alice", "bob", "charlie"}
			var scopedMember string
			mustCreateAccessList(t, client, tc.scope, name, time.Now().UTC().Add(30*24*time.Hour))
			mustCreateAccessListMember(t, client, tc.scope, name, "alice")
			mustCreateAccessListMember(t, client, tc.scope, name, "bob")
			mustCreateAccessListMember(t, client, tc.scope, name, "charlie")
			if tc.scope != "" {
				scopedMember = "/::nested"
				mustCreateAccessList(t, client, "/", "nested", time.Now().UTC().Add(30*24*time.Hour))
				_, err := runACLCommand(t, client, []string{"users", "add", "--kind=list", accessListName, scopedMember})
				require.NoError(t, err)
				removedMembers += "," + scopedMember
				expectedMembers = append(expectedMembers, scopedMember)
			}

			// Make sure we're starting with all expected members.
			out, err := runACLCommand(t, client, []string{
				"users", "ls", accessListName, "--format", "json",
			})
			require.NoError(t, err)
			require.ElementsMatch(t, expectedMembers, getMemberNames(t, out))

			// Submit a review that removes alice, bob, and the scoped list member, if present.
			_, err = runACLCommand(t, client, []string{
				"reviews", "create", accessListName,
				"--notes", "first review",
				"--remove-members", removedMembers,
			})
			require.NoError(t, err)

			// Verify the list only has charlie remaining as a member.
			out, err = runACLCommand(t, client, []string{
				"users", "ls", accessListName, "--format", "json",
			})
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"charlie"}, getMemberNames(t, out))

			// Submit second review with no changes.
			_, err = runACLCommand(t, client, []string{
				"reviews", "create", accessListName,
				"--notes", "second review",
			})
			require.NoError(t, err)

			// Verify we got both reviews.
			out, err = runACLCommand(t, client, []string{
				"reviews", "ls", accessListName, "--format", "json",
			})
			require.NoError(t, err)
			reviews := mustDecodeJSON[[]*accesslist.Review](t, out)
			require.Len(t, reviews, 2)
			require.Equal(t, tc.scope, reviews[0].GetScope())
			require.Equal(t, "second review", reviews[0].Spec.Notes)
			require.Equal(t, []string(nil), reviews[0].Spec.Changes.RemovedMembers)
			require.Equal(t, tc.scope, reviews[1].GetScope())
			require.Equal(t, "first review", reviews[1].Spec.Notes)
			require.Equal(t, []string{"alice", "bob"}, reviews[1].Spec.Changes.RemovedMembers)
			if scopedMember == "" {
				require.Empty(t, reviews[1].Spec.Changes.ScopedRemovedMembers)
			} else {
				require.Equal(t, []string{scopedMember}, reviews[1].Spec.Changes.ScopedRemovedMembers)
			}
		})
	}
}

func TestScopedACLUsersAddRemove(t *testing.T) {
	t.Parallel()
	client := setupACLSuite(t)

	const (
		parentListScope  = "/engineering/team1"
		parentListName   = "team1"
		memberListScope  = "/engineering"
		memberListName   = "engineers"
		unscopedListName = "everyone"
	)
	mustCreateAccessList(t, client, parentListScope, parentListName, time.Now().UTC().Add(30*24*time.Hour))
	mustCreateAccessList(t, client, memberListScope, memberListName, time.Now().UTC().Add(30*24*time.Hour))
	mustCreateAccessList(t, client, "", unscopedListName, time.Now().UTC().Add(30*24*time.Hour))

	parentListQualifiedName := scopes.QualifiedName{Scope: parentListScope, Name: parentListName}.String()
	memberListQualifiedName := scopes.QualifiedName{Scope: memberListScope, Name: memberListName}.String()

	for _, tc := range []struct {
		name               string
		member             string
		args               []string
		wantMembershipKind string
	}{
		{
			name:               "user",
			member:             "admin",
			wantMembershipKind: accesslist.MembershipKindUser,
		},
		{
			name:               "unscoped list",
			member:             unscopedListName,
			args:               []string{"--kind=list"},
			wantMembershipKind: accesslist.MembershipKindList,
		},
		{
			name:               "scoped list",
			member:             memberListQualifiedName,
			args:               []string{"--kind=list"},
			wantMembershipKind: accesslist.MembershipKindScopedList,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"users", "add", parentListQualifiedName, tc.member}, tc.args...)
			_, err := runACLCommand(t, client, args)
			require.NoError(t, err)

			out, err := runACLCommand(t, client, []string{"users", "ls", parentListQualifiedName, "--format=json"})
			require.NoError(t, err)
			members := mustDecodeJSON[[]*accesslist.AccessListMember](t, out)
			require.Len(t, members, 1)
			require.Equal(t, tc.member, members[0].Spec.Name)
			require.Equal(t, tc.wantMembershipKind, members[0].Spec.MembershipKind)
			require.Equal(t, parentListScope, members[0].GetScope())

			_, err = runACLCommand(t, client, []string{"users", "rm", parentListQualifiedName, tc.member})
			require.NoError(t, err)
			out, err = runACLCommand(t, client, []string{"users", "ls", parentListQualifiedName, "--format=json"})
			require.NoError(t, err)
			require.Empty(t, getMemberNames(t, out))
		})
	}
}

func TestScopedACLUpdateRemove(t *testing.T) {
	t.Parallel()
	client := setupACLSuite(t)

	const (
		name           = "test"
		scope          = "/engineering"
		accessListName = scope + "::" + name
	)
	nextAuditDate := time.Now().UTC().Add(30 * 24 * time.Hour)
	mustCreateAccessList(t, client, "", name, nextAuditDate)
	mustCreateAccessList(t, client, scope, name, nextAuditDate)

	_, err := runACLCommand(t, client, []string{"update", accessListName, "--title", "updated scoped list"})
	require.NoError(t, err)

	out, err := runACLCommand(t, client, []string{"get", accessListName, "--format=json"})
	require.NoError(t, err)
	updated := mustDecodeJSON[[]*accesslist.AccessList](t, out)
	require.Len(t, updated, 1)
	require.Equal(t, scope, updated[0].GetScope())
	require.Equal(t, "updated scoped list", updated[0].Spec.Title)

	out, err = runACLCommand(t, client, []string{"get", name, "--format=json"})
	require.NoError(t, err)
	unscoped := mustDecodeJSON[[]*accesslist.AccessList](t, out)
	require.Len(t, unscoped, 1)
	require.Equal(t, name, unscoped[0].Spec.Title)

	_, err = runACLCommand(t, client, []string{"rm", accessListName})
	require.NoError(t, err)
	_, err = runACLCommand(t, client, []string{"get", accessListName, "--format=json"})
	require.True(t, trace.IsNotFound(err), "expected NotFound after deleting scoped access list, got %v", err)

	_, err = runACLCommand(t, client, []string{"get", name, "--format=json"})
	require.NoError(t, err)
}

func TestScopedACLResourceCommands(t *testing.T) {
	t.Parallel()
	client := setupACLSuite(t)

	const (
		name  = "scoped-access-list"
		scope = "/engineering"
	)
	acl, err := accesslist.NewAccessListWithScope(header.Metadata{Name: name}, accesslist.Spec{
		Title:  "Scoped Access List",
		Owners: []accesslist.Owner{{Name: "admin"}},
	}, scope)
	require.NoError(t, err)

	yamlPath := filepath.Join(t.TempDir(), "access-list.yaml")
	yamlFile, err := os.Create(yamlPath)
	require.NoError(t, err)
	require.NoError(t, utils.WriteYAML(yamlFile, acl))
	require.NoError(t, yamlFile.Close())

	_, err = runResourceCommand(t, client, []string{"create", yamlPath})
	require.NoError(t, err)

	out, err := runResourceCommand(t, client, []string{"get", types.KindAccessList, scope + "::" + name, "--format=json"})
	require.NoError(t, err)
	accessLists := mustDecodeJSON[[]*accesslist.AccessList](t, out)
	require.Len(t, accessLists, 1)
	require.Equal(t, name, accessLists[0].GetName())
	require.Equal(t, scope, accessLists[0].GetScope())

	_, err = runResourceCommand(t, client, []string{"rm", types.KindAccessList, scope + "::" + name})
	require.NoError(t, err)

	_, err = runResourceCommand(t, client, []string{"get", types.KindAccessList, scope + "::" + name, "--format=json"})
	require.True(t, trace.IsNotFound(err), "expected NotFound after deleting scoped access list, got %v", err)
}

func setupACLSuite(t *testing.T) *authclient.Client {
	t.Helper()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			testModules := &modulestest.Modules{
				TestBuildType: modules.BuildEnterprise,
				TestFeatures: modules.Features{
					Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
						entitlements.AccessLists: {Enabled: true},
					},
				},
			}

			cfg.ScopesFeatures = scopes.Features{Enabled: true}
			cfg.PluginRegistry = plugin.NewRegistry()
			cfg.Modules = testModules
			authPlugin, err := authe.NewPlugin(authe.Config{
				License:        authe.ValidLicense{},
				LicenseChecker: authe.ValidLicense{},
				Modules:        testModules,
			})
			require.NoError(t, err)
			require.NoError(t, cfg.PluginRegistry.Add(authPlugin))
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})

	// Create an editor-level user that will act as an owner for access lists.
	user, err := types.NewUser("admin")
	user.SetRoles([]string{teleport.PresetEditorRoleName})
	require.NoError(t, err)
	_, err = process.GetAuthServer().CreateUser(t.Context(), user)
	require.NoError(t, err)

	// We need the client authenticated with the identity of an actual user for
	// some of the access list tests so we can't use testenv.NewDefaultAuthClient
	// and have to go the whole nine yards with generating user certificate.
	clt := makeClient(t, process, user.GetName())
	return clt
}

func makeClient(t *testing.T, process *service.TeleportProcess, username string) *authclient.Client {
	t.Helper()

	authAddr, err := process.AuthAddr()
	require.NoError(t, err)

	sshSigner, tlsSigner, err := cryptosuites.GenerateUserSSHAndTLSKey(t.Context(),
		cryptosuites.GetCurrentSuiteFromAuthPreference(process.GetAuthServer()))
	require.NoError(t, err)

	sshPriv, err := keys.NewPrivateKey(sshSigner)
	require.NoError(t, err)
	tlsPriv, err := keys.NewPrivateKey(tlsSigner)
	require.NoError(t, err)
	tlsPub, err := tlsPriv.MarshalTLSPublicKey()
	require.NoError(t, err)

	clusterName, err := process.GetAuthServer().GetClusterName(t.Context())
	require.NoError(t, err)
	_, tlsCert, err := process.GetAuthServer().GenerateUserTestCertsWithContext(t.Context(), auth.GenerateUserTestCertsRequest{
		SSHPubKey:      sshPriv.MarshalSSHPublicKey(),
		TLSPubKey:      tlsPub,
		Username:       username,
		TTL:            time.Hour,
		Compatibility:  constants.CertificateFormatStandard,
		RouteToCluster: clusterName.GetClusterName(),
	})
	require.NoError(t, err)

	clientCert, err := tlsPriv.TLSCertificate(tlsCert)
	require.NoError(t, err)

	caResp, err := process.GetAuthServer().GetClusterCACert(t.Context())
	require.NoError(t, err)
	rootCAs := x509.NewCertPool()
	require.True(t, rootCAs.AppendCertsFromPEM(caResp.TLSCA))

	clt, err := authclient.NewClient(apiclient.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(&tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      rootCAs,
			}),
		},
	})
	require.NoError(t, err)

	t.Cleanup(func() { clt.Close() })
	return clt
}

func mustCreateAccessList(t *testing.T, client *authclient.Client, scope, name string, nextAuditDate time.Time) {
	t.Helper()

	currentUser, err := client.GetCurrentUser(t.Context())
	require.NoError(t, err)

	acl, err := accesslist.NewAccessListWithScope(header.Metadata{Name: name}, accesslist.Spec{
		Title: name,
		Owners: []accesslist.Owner{
			{Name: currentUser.GetName()},
		},
		Audit: accesslist.Audit{
			NextAuditDate: nextAuditDate,
		},
	}, scope)
	require.NoError(t, err)

	_, err = client.AccessListClient().UpsertAccessList(t.Context(), acl)
	require.NoError(t, err)
}

func mustCreateAccessListMember(t *testing.T, client *authclient.Client, scope, accessListName, memberName string) {
	t.Helper()

	member, err := accesslist.NewAccessListMemberWithScope(header.Metadata{Name: memberName}, accesslist.AccessListMemberSpec{
		AccessList: scopes.QualifiedName{Scope: scope, Name: accessListName}.String(),
		Name:       memberName,
	}, scope)
	require.NoError(t, err)

	_, err = client.AccessListClient().UpsertAccessListMember(t.Context(), member)
	require.NoError(t, err)
}

func getMemberNames(t *testing.T, r io.Reader) []string {
	t.Helper()

	members := mustDecodeJSON[[]*accesslist.AccessListMember](t, r)
	names := make([]string, 0, len(members))
	for _, member := range members {
		names = append(names, member.GetName())
	}
	return names
}

func getAccessListNames(t *testing.T, r io.Reader) []string {
	t.Helper()

	lists := mustDecodeJSON[[]*accesslist.AccessList](t, r)
	names := make([]string, 0, len(lists))
	for _, list := range lists {
		names = append(names, scopes.QualifiedName{Scope: list.GetScope(), Name: list.GetName()}.String())
	}
	return names
}
