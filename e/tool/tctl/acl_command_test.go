package main_test

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"testing"
	"time"

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
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMain(m *testing.M) {
	modules.SetModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.AccessLists: {Enabled: true},
			},
		},
	})
	os.Exit(m.Run())
}

// TestACLList tests access lists CLI for displaying access lists.
func TestACLList(t *testing.T) {
	client := setupACLSuite(t)

	mustCreateAccessList(t, client, "past-due", time.Now().UTC().Add(-24*time.Hour))
	mustCreateAccessList(t, client, "due", time.Now().UTC().Add(24*time.Hour))
	mustCreateAccessList(t, client, "not-yet-due", time.Now().UTC().Add(30*24*time.Hour))

	// Fetch all lists.
	out, err := runACLCommand(t, client, []string{
		"ls", "--format", "json",
	})
	require.NoError(t, err)
	allLists := getAccessListNames(t, out)
	require.ElementsMatch(t, []string{"past-due", "due", "not-yet-due"}, allLists)

	// Fetch only lists that are due a review.
	out, err = runACLCommand(t, client, []string{
		"ls", "--review-only", "--format", "json",
	})
	require.NoError(t, err)
	reviewOnlyLists := getAccessListNames(t, out)
	require.ElementsMatch(t, []string{"past-due", "due"}, reviewOnlyLists)
}

// TestACLReviews tests access lists CLI for managing access list reviews.
func TestACLReviews(t *testing.T) {
	client := setupACLSuite(t)

	const accessListName = "test"
	mustCreateAccessList(t, client, accessListName, time.Now().UTC().Add(30*24*time.Hour))
	mustCreateAccessListMember(t, client, accessListName, "alice")
	mustCreateAccessListMember(t, client, accessListName, "bob")
	mustCreateAccessListMember(t, client, accessListName, "charlie")

	// Make sure we're indeed starting with 3 members.
	out, err := runACLCommand(t, client, []string{
		"users", "ls", accessListName, "--format", "json",
	})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"alice", "bob", "charlie"}, getMemberNames(t, out))

	// Submit a review that removes alice and bob.
	_, err = runACLCommand(t, client, []string{
		"reviews", "create", accessListName,
		"--notes", "first review",
		"--remove-members", "alice,bob",
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
	require.Equal(t, "second review", reviews[0].Spec.Notes)
	require.Equal(t, []string(nil), reviews[0].Spec.Changes.RemovedMembers)
	require.Equal(t, "first review", reviews[1].Spec.Notes)
	require.Equal(t, []string{"alice", "bob"}, reviews[1].Spec.Changes.RemovedMembers)
}

func setupACLSuite(t *testing.T) *authclient.Client {
	t.Helper()

	process, err := testenv.NewTeleportProcess(
		t.TempDir(),
		testenv.WithConfig(func(cfg *servicecfg.Config) {
			cfg.PluginRegistry = plugin.NewRegistry()
			authPlugin, err := authe.NewPlugin(authe.Config{
				License: authe.ValidLicense{},
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

func mustCreateAccessList(t *testing.T, client *authclient.Client, name string, nextAuditDate time.Time) {
	t.Helper()

	currentUser, err := client.GetCurrentUser(t.Context())
	require.NoError(t, err)

	acl, err := accesslist.NewAccessList(header.Metadata{Name: name}, accesslist.Spec{
		Title: name,
		Owners: []accesslist.Owner{
			{Name: currentUser.GetName()},
		},
		Audit: accesslist.Audit{
			NextAuditDate: nextAuditDate,
		},
	})
	require.NoError(t, err)

	_, err = client.AccessListClient().UpsertAccessList(t.Context(), acl)
	require.NoError(t, err)
}

func mustCreateAccessListMember(t *testing.T, client *authclient.Client, accessListName, memberName string) {
	t.Helper()

	member, err := accesslist.NewAccessListMember(header.Metadata{Name: memberName}, accesslist.AccessListMemberSpec{
		AccessList: accessListName,
		Name:       memberName,
	})
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
		names = append(names, list.GetName())
	}
	return names
}
