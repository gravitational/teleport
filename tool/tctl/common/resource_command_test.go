package common

import (
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/services"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func mockCreate(client auth.ClientI, raw services.UnknownResource) error {
	return nil
}

func TestResourceCommand_DecodeResources(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "auth_command_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	remoteCluster, err := types.NewRemoteCluster("leaf.example.com")
	if err != nil {
		t.Fatal(err)
	}

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{PublicKey: []byte("SSH CA cert")}},
			TLS: []*types.TLSKeyPair{{Cert: []byte("TLS CA cert")}},
		},
		Roles:      nil,
		SigningAlg: types.CertAuthoritySpecV2_RSA_SHA2_512,
	})
	require.NoError(t, err)

	client := mockClient{
		clusterName:    clusterName,
		remoteClusters: []types.RemoteCluster{remoteCluster},
		userCerts: &proto.Certs{
			SSH: []byte("SSH cert"),
			TLS: []byte("TLS cert"),
		},
		cas: []types.CertAuthority{ca},
		proxies: []types.Server{
			&types.ServerV2{
				Kind:    types.KindNode,
				Version: types.V2,
				Metadata: types.Metadata{
					Name: "proxy",
				},
				Spec: types.ServerSpecV2{
					PublicAddr: "proxy-from-api.example.com:3080",
				},
			},
		},
	}
	resourcecreatehandlers := map[ResourceKind]ResourceCreateHandler{
		types.KindUser:                    mockCreate,
		types.KindRole:                    mockCreate,
		types.KindTrustedCluster:          mockCreate,
		types.KindGithubConnector:         mockCreate,
		types.KindCertAuthority:           mockCreate,
		types.KindClusterAuthPreference:   mockCreate,
		types.KindClusterNetworkingConfig: mockCreate,
		types.KindSessionRecordingConfig:  mockCreate,
	}
	rc := ResourceCommand{
		config:         nil,
		ref:            services.Ref{},
		refs:           nil,
		format:         "",
		namespace:      "",
		withSecrets:    false,
		force:          false,
		confirm:        false,
		ttl:            "",
		labels:         "",
		filename:       "",
		deleteCmd:      nil,
		getCmd:         nil,
		createCmd:      nil,
		updateCmd:      nil,
		CreateHandlers: resourcecreatehandlers,
	}

	blank := strings.NewReader("")
	err = rc.DecodeResources(client, blank)
	require.Error(t, err, "return error on blank input")

	blank2 := strings.NewReader(" ")
	err = rc.DecodeResources(client, blank2)
	require.Error(t, err, "return error on whitespace only input")

	bogus := strings.NewReader("kind: bogus\n---")
	err = rc.DecodeResources(client, bogus)
	require.Error(t, err, "return error on bogus kind")

	multi := strings.NewReader("version: v3\nkind: role\nmetadata:\n  name: admin\nspec:\n  allow:\n    node_labels:\n      '*': '*'\n---\nkind: role\nmetadata:\n  name: admin2\n")
	err = rc.DecodeResources(client, multi)
	require.NoError(t, err, "create multiple roles")

	// Fixes #4703
	blanklineyaml := strings.NewReader("\n---\nkind: role\nmetadata:\n  name: admin3")
	err = rc.DecodeResources(client, blanklineyaml)
	require.NoError(t, err, "a blank line above a yaml separator must not cause a failure")

}
