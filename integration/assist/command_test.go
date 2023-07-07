package assist

// NOTE: this test requires running with `-tags "webassets_embed"` for now.

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/types"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/integration/helpers"
	auth2 "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

// TODO: mock openAI
// TODO: mock SSH command output
// TODO: check the session recording has been uplodaded

const testUser = "fullaccess"

type testIdentity struct {
	*identityfile.IdentityFile
}

func (_ testIdentity) Dialer(cfg client.Config) (client.ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

func newTestIdentity(key *rsa.PrivateKey, sshCert, tlsCert []byte, authority types.CertAuthority) testIdentity {
	caKeySet := authority.GetActiveKeys()
	var sshCACerts, tlsCACerts [][]byte
	for _, k := range caKeySet.SSH {
		sshCACerts = append(sshCACerts, k.PublicKey)
	}
	for _, k := range caKeySet.TLS {
		tlsCACerts = append(tlsCACerts, k.Cert)
	}
	pemKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	return testIdentity{
		&identityfile.IdentityFile{
			PrivateKey: pemKey,
			Certs: identityfile.Certs{
				SSH: sshCert,
				TLS: tlsCert,
			},
			CACerts: identityfile.CACerts{
				SSH: [][]byte{},
				TLS: tlsCACerts,
			},
		},
	}

}

// TestIntegrationCRUD starts a Teleport cluster and using its Proxy Web server,
// tests the CRUD operations over the Integration resource.
func TestAssistCommandOpenSSH(t *testing.T) {
	testDir := t.TempDir()

	// Setup: starting a Teleport instance
	ctx := context.Background()
	clusterName := "root.example.com"

	cfg := helpers.InstanceConfig{
		ClusterName: clusterName,
		HostID:      uuid.New().String(),
		NodeName:    helpers.Loopback,
		Log:         utils.NewLoggerForTests(),
	}
	cfg.Listeners = helpers.SingleProxyPortSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)

	var err error
	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = testDir
	rcConf.Auth.Enabled = true
	rcConf.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = false
	rcConf.SSH.Enabled = false
	rcConf.Version = "v3"
	rcConf.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{types.RoleNode},
				Token: "token",
			},
		},
	})
	rcConf.Proxy.AssistAPIKey = "test"
	rcConf.Auth.AssistAPIKey = "test"
	require.NoError(t, err)
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	defer rc.StopAll()

	auth := rc.Process.GetAuthServer()
	proxyAddr, err := rc.Process.ProxyWebAddr()
	require.NoError(t, err)
	t.Log(proxyAddr)

	// Test setup: we create a role that gives full access and allows login as
	// `testUser` so that the user is able to login to the SSH node and execute
	// commands through assist
	roleWithFullAccess, err := types.NewRole(testUser, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Namespaces: []string{apidefaults.Namespace},
			Rules: []types.Rule{
				types.NewRule(types.Wildcard, services.RW()),
			},
			// TODO: autodetect to have a working SSH server?
			// This might not be required depending on how we mock the SSH
			// server
			Logins: []string{testUser},
		},
	})
	require.NoError(t, auth.UpsertRole(ctx, roleWithFullAccess))

	// Test setup: create the user and set its password (this is required for web login)
	user, err := types.NewUser(testUser)
	require.NoError(t, err)

	user.AddRole(roleWithFullAccess.GetName())
	require.NoError(t, auth.UpsertUser(user))

	userPassword := uuid.NewString()
	require.NoError(t, auth.UpsertPassword(testUser, []byte(userPassword)))

	// Test setup: WIP - broken
	// Get a working auth client for the test user
	// to call `client.CreateAssistantConversation()`
	userKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	userPubKey, err := ssh.NewPublicKey(&userKey.PublicKey)
	require.NoError(t, err)
	testCertsReq := auth2.GenerateUserTestCertsRequest{
		Key:            ssh.MarshalAuthorizedKey(userPubKey),
		Username:       user.GetName(),
		TTL:            time.Hour,
		Compatibility:  constants.CertificateFormatStandard,
		RouteToCluster: clusterName,
	}
	sshCert, tlsCert, err := auth.GenerateUserTestCerts(testCertsReq)
	require.NoError(t, err)
	authority, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, false)
	require.NoError(t, err)

	clientConfig := client.Config{
		Addrs:                    []string{proxyAddr.String()},
		Credentials:              []client.Credentials{newTestIdentity(userKey, sshCert, tlsCert, authority)},
		InsecureAddressDiscovery: true,
		DialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	}
	userClient, err := client.New(ctx, clientConfig)
	require.NoError(t, err)
	_, err = userClient.Ping(ctx)
	require.NoError(t, err)

	// End of broken part, here we use the admin auth client, it's working but
	// will not have the correct login trait to exec on the SSH node
	authClient := rc.GetSiteAPI(rc.Secrets.SiteName)

	// Setup: creating an openssh mock server
	ca, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName,
	}, true)
	require.NoError(t, err)

	signers, err := sshutils.GetSigners(ca)
	require.NoError(t, err)
	require.Len(t, signers, 1)

	cert, err := apisshutils.MakeRealHostCert(signers[0])
	require.NoError(t, err)
	handler := sshutils.NewChanHandlerFunc(func(_ context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
		ch, _, err := nch.Accept()
		require.NoError(t, err)
		require.NoError(t, ch.Close())
	})
	sshServer, err := sshutils.NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		handler,
		[]ssh.Signer{cert},
		sshutils.AuthMethods{NoClient: true},
		sshutils.SetInsecureSkipHostValidation(),
	)
	assert.NoError(t, sshServer.Start())
	time.Sleep(3 * time.Second)
	sshAddr := sshServer.Addr()
	t.Log(sshAddr)

	// Setup: running a one-shot Teleport instance to register our mock SSH node
	// into the cluster and allow agentless execution.
	opensshConfigPath := filepath.Join(testDir, "sshd_config")
	f, err := os.Create(opensshConfigPath)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	teleportDataDir := filepath.Join(testDir, "teleport_openssh")

	openSSHCfg := servicecfg.MakeDefaultConfig()

	openSSHCfg.OpenSSH.Enabled = true
	err = config.ConfigureOpenSSH(&config.CommandLineFlags{
		DataDir:           teleportDataDir,
		ProxyServer:       rc.Web,
		AuthToken:         "token",
		JoinMethod:        string(types.JoinMethodToken),
		OpenSSHConfigPath: opensshConfigPath,
		RestartOpenSSH:    false,
		CheckCommand:      "echo okay",
		Labels:            "hello=true",
		Address:           sshAddr,
		InsecureMode:      true,
		Debug:             true,
	}, openSSHCfg)
	require.NoError(t, err)

	err = service.Run(ctx, *openSSHCfg, nil)
	require.NoError(t, err)

	// check a node with the flags specified exists
	// TODO: once RBAC is fixed, use helpers.FindNodeWithLabel instead of a sleep()
	time.Sleep(3 * time.Second)
	nodes, err := authClient.GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodes))
	node := nodes[0]

	// Test: creating a new conversation
	conversation, err := authClient.CreateAssistantConversation(context.Background(), &assist.CreateAssistantConversationRequest{
		Username:    "fullaccess",
		CreatedTime: timestamppb.Now(),
	})
	// Test: executing a command

	webPack := helpers.LoginWebClient(t, proxyAddr.String(), testUser, userPassword)
	endpoint, err := url.JoinPath("command", "$site", "execute")
	require.NoError(t, err)

	req := web.CommandRequest{
		Query:          fmt.Sprintf("name == \"%s\"", node.GetName()),
		Login:          testUser,
		ConversationID: conversation.Id,
		ExecutionID:    uuid.New().String(),
		Command:        "echo teleport",
	}

	// Currently this fails because we are logged in as Admin, which has no
	// RBAC rule nor trait allowing it to exec on the registered node.
	ws, resp, err := webPack.OpenWebsocket(t, endpoint, req)
	require.NoError(t, err)
	resp.Body.Close()

	_, _, err = ws.ReadMessage()
	require.NoError(t, err)
}
