package common

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMCPDBCommand(t *testing.T) {
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"postgres"})
	alice.SetDatabaseNames([]string{"postgres"})
	alice.SetRoles([]string{"access"})

	authProcess := testserver.MakeTestServer(
		t,
		testserver.WithBootstrap(connector, alice),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres-local",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "mysql-local",
					Protocol: defaults.ProtocolMySQL,
					URI:      "external-mysql:3306",
				},
			}
		}),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	err = Run(t.Context(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	stdin, writer := io.Pipe()
	reader, stdout := io.Pipe()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	executionCh := make(chan error)
	go func() {
		executionCh <- Run(ctx, []string{
			"mcp", "db", "--insecure", "--db-user=postgres", "--db-name=postgres",
		}, setHomePath(tmpHomePath), func(c *CLIConf) error {
			c.overrideStdin = stdin
			c.OverrideStdout = stdout
			// MCP server logs are going to be discarded.
			c.overrideStderr = io.Discard
			c.databaseMCPRegistryOverride = map[string]dbmcp.NewServerFunc{
				defaults.ProtocolPostgres: func(ctx context.Context, nsc *dbmcp.NewServerConfig) (dbmcp.Server, error) {
					return &testDatabaseMCP{}, nil
				},
			}
			return nil
		})
	}()

	clt := mcpclient.NewClient(NewStdio(writer, bufio.NewReader(reader)))
	require.NoError(t, clt.Start(t.Context()))

	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {

		_, err = clt.Initialize(t.Context(), req)
		require.NoError(collect, err)
		require.NoError(collect, clt.Ping(t.Context()))
	}, time.Second, 100*time.Millisecond)

	// Stop the MCP server command and wait until it is finshed.
	cancel()
	select {
	case err := <-executionCh:
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		require.Fail(t, "expected the execution to be completed")
	}
}

// testDatabaseMCP is a noop database MCP server.
type testDatabaseMCP struct{}

func (s *testDatabaseMCP) Close(_ context.Context) error { return nil }

// Stdio is a MCP client transport.
//
// Extracted from mcp-go library with changes to use in-memory in/out instead
// of relying on starting a subcommand.
type Stdio struct {
	stdin          io.WriteCloser
	stdout         *bufio.Reader
	responses      map[int64]chan *mcptransport.JSONRPCResponse
	mu             sync.RWMutex
	done           chan struct{}
	onNotification func(mcp.JSONRPCNotification)
	notifyMu       sync.RWMutex
}

func NewStdio(in io.WriteCloser, out *bufio.Reader) mcptransport.Interface {
	return &Stdio{
		stdin:     in,
		stdout:    out,
		responses: make(map[int64]chan *mcptransport.JSONRPCResponse),
		done:      make(chan struct{}),
	}
}

func (c *Stdio) Start(ctx context.Context) error {
	go func() {
		c.readResponses()
	}()

	return nil
}

// Close shuts down the stdio client, closing the stdin pipe and waiting for the subprocess to exit.
// Returns an error if there are issues closing stdin or waiting for the subprocess to terminate.
func (c *Stdio) Close() error {
	close(c.done)
	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	return nil
}

// OnNotification registers a handler function to be called when notifications are received.
// Multiple handlers can be registered and will be called in the order they were added.
func (c *Stdio) SetNotificationHandler(
	handler func(notification mcp.JSONRPCNotification),
) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	c.onNotification = handler
}

// readResponses continuously reads and processes responses from the server's stdout.
// It handles both responses to requests and notifications, routing them appropriately.
// Runs until the done channel is closed or an error occurs reading from stdout.
func (c *Stdio) readResponses() {
	for {
		select {
		case <-c.done:
			return
		default:
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading response: %v\n", err)
				}
				return
			}

			var baseMessage mcptransport.JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &baseMessage); err != nil {
				continue
			}

			// Handle notification
			if baseMessage.ID == nil {
				var notification mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(line), &notification); err != nil {
					continue
				}
				c.notifyMu.RLock()
				if c.onNotification != nil {
					c.onNotification(notification)
				}
				c.notifyMu.RUnlock()
				continue
			}

			c.mu.RLock()
			ch, ok := c.responses[*baseMessage.ID]
			c.mu.RUnlock()

			if ok {
				ch <- &baseMessage
				c.mu.Lock()
				delete(c.responses, *baseMessage.ID)
				c.mu.Unlock()
			}
		}
	}
}

// sendRequest sends a JSON-RPC request to the server and waits for a response.
// It creates a unique request ID, sends the request over stdin, and waits for
// the corresponding response or context cancellation.
// Returns the raw JSON response message or an error if the request fails.
func (c *Stdio) SendRequest(
	ctx context.Context,
	request mcptransport.JSONRPCRequest,
) (*mcptransport.JSONRPCResponse, error) {
	if c.stdin == nil {
		return nil, fmt.Errorf("stdio client not started")
	}

	// Create the complete request structure
	responseChan := make(chan *mcptransport.JSONRPCResponse, 1)
	c.mu.Lock()
	c.responses[request.ID] = responseChan
	c.mu.Unlock()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	if _, err := c.stdin.Write(requestBytes); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.responses, request.ID)
		c.mu.Unlock()
		return nil, ctx.Err()
	case response := <-responseChan:
		return response, nil
	}
}

// SendNotification sends a json RPC Notification to the server.
func (c *Stdio) SendNotification(
	ctx context.Context,
	notification mcp.JSONRPCNotification,
) error {
	if c.stdin == nil {
		return fmt.Errorf("stdio client not started")
	}

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	notificationBytes = append(notificationBytes, '\n')

	if _, err := c.stdin.Write(notificationBytes); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}
