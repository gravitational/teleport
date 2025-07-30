package tbot

import (
	"cmp"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

func MCPServiceBuilder(botCfg *config.BotConfig) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		svc := &MCPService{
			botAuthClient:      deps.Client,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			botCfg:             botCfg,
			reloadCh:           deps.ReloadCh,
			identityGenerator:  deps.IdentityGenerator,
			clientBuilder:      deps.ClientBuilder,
		}
		svc.log = deps.Logger.With(
			teleport.ComponentKey,
			teleport.Component(teleport.ComponentTBot, "svc", svc.String()),
		)
		svc.statusReporter = deps.StatusRegistry.AddService(svc.String())
		return svc, nil
	}
}

type MCPService struct {
	// botAuthClient should be an auth client using the bots internal identity.
	// This will not have any roles impersonated and should only be used to
	// fetch CAs.
	botAuthClient      *apiclient.Client
	botIdentityReadyCh <-chan struct{}
	botCfg             *config.BotConfig
	log                *slog.Logger
	statusReporter     readyz.Reporter
	reloadCh           <-chan struct{}
	identityGenerator  *identity.Generator
	clientBuilder      *client.Builder
}

func (s *MCPService) String() string {
	return cmp.Or(
		"mcp",
	)
}

func (s *MCPService) Run(ctx context.Context) error {
	id, err := s.identityGenerator.GenerateFacade(
		ctx,
		identity.WithLogger(s.log),
		identity.WithLifetime(time.Hour, time.Minute),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	c, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return trace.Wrap(err)
	}

	server := mcpserver.NewMCPServer(
		"tbot",
		"v1.0.0",
	)

	server.AddTool(mcp.NewTool(
		"list_servers",
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		s.log.InfoContext(ctx, "Handling MCP tool request")
		nodeList, err := c.GetNodes(ctx, "default")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		marshalledNodes, err := json.MarshalIndent(
			nodeList, "", "  ",
		)
		if err != nil {
			return nil, trace.Wrap(err, "marshalling nodes")
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(marshalledNodes)),
			},
		}, nil
	})

	server.AddTool(mcp.NewTool(
		"run_ssh_command",
		mcp.WithDescription("Run a command on a server via SSH"),
		mcp.WithString("hostname",
			mcp.Required(),
			mcp.Description("Hostname of the server to connect to"),
		),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("Command to run on a server"),
		),
		mcp.WithString(
			"username",
			mcp.Required(),
			mcp.Description("Username on the server to connect as"),
		),
	), func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		hostnameParam, err := request.RequireString("hostname")
		if err != nil {
			return nil, trace.Wrap(err, "missing hostname")
		}
		commandParam, err := request.RequireString("command")
		if err != nil {
			return nil, trace.Wrap(err, "missing command")
		}
		usernameParam, err := request.RequireString("username")
		if err != nil {
			return nil, trace.Wrap(err, "missing username")
		}

		sshConfig, err := id.SSHClientConfig()
		if err != nil {
			return nil, trace.Wrap(err, "getting SSH client config")
		}
		sshConfig.User = usernameParam
		proxyCfg := proxy.ClientConfig{
			ProxyAddress:      s.botCfg.ProxyServer,
			TLSRoutingEnabled: true,
			SSHConfig:         sshConfig,
			TLSConfigFunc: func(cluster string) (*tls.Config, error) {
				tlsConfig, err := id.TLSConfig()
				if err != nil {
					return nil, fmt.Errorf("getting TLS config from credentials: %w", err)
				}
				// Set the server name to the target host for SNI.
				tlsConfig.ServerName = cluster
				// Blank out NextProtos to delegate setting this to proxy.Client.
				tlsConfig.NextProtos = nil
				return tlsConfig, nil
			},
		}
		proxyClient, err := proxy.NewClient(ctx, proxyCfg)
		if err != nil {
			return nil, trace.Wrap(err, "creating proxy client")
		}
		defer proxyClient.Close()
		// Default to targetting the cluster of the bot identity.
		targetCluster := id.Get().ClusterName
		targetHostPort := hostnameParam + ":0"
		conn, _, err := proxyClient.DialHost(
			ctx,
			targetHostPort,
			targetCluster,
			nil,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"dialing host %q in cluster %q: %w",
				targetHostPort, targetCluster, err,
			)
		}

		sshConn, newCh, newReq, err := ssh.NewClientConn(
			conn, targetHostPort, sshConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("creating SSH client connection: %w", err)
		}
		sshClient := ssh.NewClient(sshConn, newCh, newReq)
		defer sshClient.Close()

		sess, err := sshClient.NewSession()
		if err != nil {
			return nil, fmt.Errorf("creating SSH session: %w", err)
		}
		defer sess.Close()

		output, err := sess.CombinedOutput(commandParam)
		if err != nil {
			return nil, fmt.Errorf("running command %q on host %q: %w", commandParam, hostnameParam, err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(string(output)),
			},
		}, nil
	})

	s.log.InfoContext(ctx, "MCP Server about to listen")
	httpServer := mcpserver.NewSSEServer(server)
	context.AfterFunc(ctx, func() {
		httpServer.Shutdown(context.Background())
	})
	return httpServer.Start(":1338")
}
