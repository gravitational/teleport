/*

 Copyright 2023 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.

*/

package assist

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/fs"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/ai/testutils"
	libauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/teleport/lib/web/terminal"
)

const (
	testUser          = "testUser"
	testCommandOutput = "teleport1234"
	testToken         = "token"
	testClusterName   = "teleport.example.com"
)

// TestAssistCommandOpenSSH tests that command output is properly recorded when
// executing commands through assist on OpenSSH nodes.
func TestAssistCommandOpenSSH(t *testing.T) {
	// Setup section: starting Teleport, creating the user and starting a mock SSH server
	testDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	openAIMock := mockOpenAI(t)

	rc := setupTeleport(t, testDir, openAIMock.URL)
	proxyAddr, err := rc.Process.ProxyWebAddr()
	require.NoError(t, err)

	userClient, userPassword := setupTestUser(t, ctx, rc)

	node := registerAndSetupMockSSHNode(t, ctx, testDir, rc)

	// Test section: We're checking that when a user executes a command through
	// Assist on an agentless node, a session recording gets created and
	// contains the command output.

	// Create a new conversation
	conversation, err := userClient.CreateAssistantConversation(context.Background(), &assist.CreateAssistantConversationRequest{
		Username:    testUser,
		CreatedTime: timestamppb.Now(),
	})
	require.NoError(t, err)

	// Login and execute the command
	webPack := helpers.LoginWebClient(t, proxyAddr.String(), testUser, userPassword)
	endpoint, err := url.JoinPath("command", "$site", "execute")
	require.NoError(t, err)

	req := web.CommandRequest{
		Query:          fmt.Sprintf("name == \"%s\"", node.GetHostname()),
		Login:          testUser,
		ConversationID: conversation.Id,
		ExecutionID:    uuid.New().String(),
		Command:        "echo teleport",
	}

	ws, resp, err := webPack.OpenWebsocket(t, endpoint, req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	// Processing the execution websocket messages:
	// - the first message is the session metadata (including the session ID we need)
	// - the second message is the streamed command output
	// - the third message is a session close

	execSocket := executionWebsocketReader{ws}

	// First message: session metadata
	envelope, err := execSocket.Read()
	require.NoError(t, err)
	var sessionMetadata sessionMetadataResponse
	require.NoError(t, json.Unmarshal([]byte(envelope.Payload), &sessionMetadata))

	// Second message: command output
	envelope, err = execSocket.Read()
	require.NoError(t, err)
	perNodeEnvelope := struct {
		ID      string `json:"node_id"`
		MsgType string `json:"type"`
		Payload string `json:"payload"`
	}{}
	require.NoError(t, json.Unmarshal([]byte(envelope.Payload), &perNodeEnvelope))
	// Assert the command executed properly. If the execution failed, we will
	// receive a web.envelopeTypeError message instead
	require.Equal(t, web.EnvelopeTypeStdout, perNodeEnvelope.MsgType)
	output, err := base64.StdEncoding.DecodeString(perNodeEnvelope.Payload)
	require.NoError(t, err)
	// Assert the streamed command output content is the one expected
	require.Equal(t, testCommandOutput, string(output))

	// Third message: session close
	envelope, err = execSocket.Read()
	require.NoError(t, err)
	require.Equal(t, defaults.WebsocketClose, envelope.Type)
	// Now the execution is finished
}

// mockOpenAI starts an OpenAI mock server that answers one completion request
// successfully (the output is a plain text command summary, it cannot be used
// for an agent thinking step.
// The server returns errors for embeddings requests from the auth, but
// this should not affect the test.
func mockOpenAI(t *testing.T) *httptest.Server {
	responses := []string{"This is the summary of the command."}
	server := httptest.NewServer(testutils.GetTestHandlerFn(t, responses))
	t.Cleanup(server.Close)
	return server
}

// setupTeleport starts a Teleport instance running the Auth and Proxy service,
// with Assist and the web service enabled. The instance supports Node joining
// with the static token testToken.
func setupTeleport(t *testing.T, testDir, openaiMockURL string) *helpers.TeleInstance {
	cfg := helpers.InstanceConfig{
		ClusterName: testClusterName,
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
	rcConf.Proxy.DisableWebService = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = false
	rcConf.Version = "v3"
	rcConf.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{types.RoleNode},
				Token: testToken,
			},
		},
	})
	rcConf.Proxy.AssistAPIKey = "test"
	rcConf.Auth.AssistAPIKey = "test"
	openAIConfig := openai.DefaultConfig("test")
	openAIConfig.BaseURL = openaiMockURL + "/v1"
	rcConf.Testing.OpenAIConfig = &openAIConfig
	require.NoError(t, err)
	rcConf.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rc.StopAll()
	})

	return rc
}

// setupTestUser creates a user with the access, editor and auditor roles. This
// user must be able to execute commands on the test SSH node, and query the
// session recordings.
// The function also sets a password for the user (this is needed to log in and
// call the web endpoints).
// Finally, it also builds and returns a Teleport client logged in as the user.
func setupTestUser(t *testing.T, ctx context.Context, rc *helpers.TeleInstance) (*client.Client, string) {
	auth := rc.Process.GetAuthServer()
	// Create user
	user, err := types.NewUser(testUser)
	require.NoError(t, err)
	user.SetLogins([]string{testUser})
	user.AddRole(teleport.PresetEditorRoleName)
	user.AddRole(teleport.PresetAccessRoleName)
	user.AddRole(teleport.PresetAuditorRoleName)
	require.NoError(t, auth.UpsertUser(user))

	userPassword := uuid.NewString()
	require.NoError(t, auth.UpsertPassword(testUser, []byte(userPassword)))

	creds, err := newTestCredentials(t, rc, user)
	require.NoError(t, err)
	clientConfig := client.Config{
		Addrs:       []string{rc.Auth},
		Credentials: []client.Credentials{creds},
	}
	userClient, err := client.New(ctx, clientConfig)
	require.NoError(t, err)
	_, err = userClient.Ping(ctx)
	require.NoError(t, err)

	return userClient, userPassword
}

// newTestCredentials builds Teleport credentials for the testUser.
// Those credentials can only be used for auth connection.
func newTestCredentials(t *testing.T, rc *helpers.TeleInstance, user types.User) (client.Credentials, error) {
	auth := rc.Process.GetAuthServer()

	// Get user certs
	userKey, err := native.GenerateRSAPrivateKey()
	require.NoError(t, err)
	userPubKey, err := ssh.NewPublicKey(&userKey.PublicKey)
	require.NoError(t, err)
	testCertsReq := libauth.GenerateUserTestCertsRequest{
		Key:            ssh.MarshalAuthorizedKey(userPubKey),
		Username:       user.GetName(),
		TTL:            time.Hour,
		Compatibility:  constants.CertificateFormatStandard,
		RouteToCluster: testClusterName,
	}
	_, tlsCert, err := auth.GenerateUserTestCerts(testCertsReq)
	require.NoError(t, err)

	// Build credentials from the certs
	pemKey := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(userKey),
		},
	)
	cert, err := keys.X509KeyPair(tlsCert, pemKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(rc.Secrets.TLSHostCACert)
	pool.AppendCertsFromPEM(rc.Secrets.TLSUserCACert)

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}
	return client.LoadTLS(tlsConf), nil
}

// registerAndSetupMockSSHNode registers an agentless SSH node in Teleport and
// starts a mock SSH server.
func registerAndSetupMockSSHNode(t *testing.T, ctx context.Context, testDir string, rc *helpers.TeleInstance) types.Server {
	// Reserve the listener for the SSH server. We can't start the SSH server
	// right now because we need to get a valid certificate. The certificate
	// needs proper principals, which implies knowing the node ID. This only
	// happens after the node has joined.
	var sshListenerFds []*servicecfg.FileDescriptor
	sshAddr := helpers.NewListenerOn(t, "localhost", service.ListenerNodeSSH, &sshListenerFds)

	node := registerMockSSHNode(t, ctx, sshAddr, testDir, rc)

	sshListener, err := sshListenerFds[0].ToListener()
	require.NoError(t, err)

	setupMockSSHNode(t, ctx, sshListener, node.GetName(), rc)

	return node
}

func registerMockSSHNode(t *testing.T, ctx context.Context, sshAddr, testDir string, rc *helpers.TeleInstance) types.Server {
	// Setup: running a one-shot Teleport instance to register our mock SSH node
	// into the cluster and allow agentless execution.
	opensshConfigPath := filepath.Join(testDir, "sshd_config")
	require.NoError(t, os.WriteFile(opensshConfigPath, []byte{}, fs.FileMode(0644)))
	teleportDataDir := filepath.Join(testDir, "teleport_openssh")

	openSSHCfg := servicecfg.MakeDefaultConfig()

	openSSHCfg.OpenSSH.Enabled = true
	err := config.ConfigureOpenSSH(&config.CommandLineFlags{
		DataDir:           teleportDataDir,
		ProxyServer:       rc.Web,
		AuthToken:         testToken,
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

	// Wait for node propagation
	require.Eventually(t, helpers.FindNodeWithLabel(t, ctx, rc.Process.GetAuthServer(), "hello", "true"), time.Second*2, time.Millisecond*50)
	nodes, err := rc.Process.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
	require.NoError(t, err)
	require.Equal(t, 1, len(nodes))
	return nodes[0]
}

func setupMockSSHNode(t *testing.T, ctx context.Context, sshListener net.Listener, nodeName string, rc *helpers.TeleInstance) {
	// Setup: creating and starting openssh mock server
	ca, err := rc.Process.GetAuthServer().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: testClusterName,
	}, true)
	require.NoError(t, err)

	signers, err := sshutils.GetSigners(ca)
	require.NoError(t, err)
	require.Len(t, signers, 1)

	cert, err := apisshutils.MakeRealHostCertWithPrincipals(signers[0], nodeName)
	require.NoError(t, err)
	handler := sshutils.NewChanHandlerFunc(handlerSSH)
	sshServer, err := sshutils.NewServer(
		"test",
		utils.NetAddr{AddrNetwork: "tcp", Addr: sshListener.Addr().String()},
		handler,
		[]ssh.Signer{cert},
		sshutils.AuthMethods{
			NoClient: true,
		},
		sshutils.SetInsecureSkipHostValidation(),
		sshutils.SetLogger(utils.NewLoggerForTests().WithField("component", "mocksshserver")),
	)
	require.NoError(t, err)
	require.NoError(t, sshServer.SetListener(sshListener))
	require.NoError(t, sshServer.Start())
}

// this is a dummy SSH handler. It only supports "exec" requests. All other
// requests are happily acknowledged and discarded. Receieving an "exec" request
// sends testCommandOutput in the main channel and closes all channels.
// This is not strictly following the SSH RFC as request processing is blocked
// as soon as an exec request is received, but is good enough for our use-case.
func handlerSSH(_ context.Context, ccx *sshutils.ConnectionContext, nch ssh.NewChannel) {
	ch, requests, err := nch.Accept()
	if err != nil {
		return
	}
	// Sessions have out-of-band requests such as "shell",
	// "pty-req", "env" and "exec". Here we don't output anything and start a
	// routine consuming requests and waiting for the "exec" one.
	go func(in <-chan *ssh.Request) {
		for {
			select {
			case req := <-in:
				if req.Type == "exec" {
					req.Reply(true, nil)
					_, err = ch.Write([]byte(testCommandOutput))
					msg := struct {
						Status uint32
					}{
						Status: 0,
					}
					ch.SendRequest("exit-status", false, ssh.Marshal(&msg))
					ch.Close()
					ccx.Close()
					return
				} else {
					req.Reply(true, nil)
				}
			// If it's been 10 seconds we have not received any message, we exit
			case <-time.After(10 * time.Second):
				ch.Close()
				ccx.Close()
				return
			}

		}
	}(requests)
}

// Small helper that wraps a websocket and unmarshalls messages as Teleport
// websocket ones.
type executionWebsocketReader struct {
	*websocket.Conn
}

func (r executionWebsocketReader) Read() (terminal.Envelope, error) {
	_, data, err := r.ReadMessage()
	if err != nil {
		return terminal.Envelope{}, trace.Wrap(err)
	}
	var envelope terminal.Envelope
	return envelope, trace.Wrap(proto.Unmarshal(data, &envelope))
}

// This is used for unmarshalling
type sessionMetadataResponse struct {
	Session session.Session `json:"session"`
}
