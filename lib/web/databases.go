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

package web

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	dbrepl "github.com/gravitational/teleport/lib/client/db/repl"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/ui"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web/scripts"
	"github.com/gravitational/teleport/lib/web/terminal"
	webui "github.com/gravitational/teleport/lib/web/ui"
)

// createOrOverwriteDatabaseRequest contains the necessary basic information
// to create (or overwrite) a database.
// Database here is the database resource, containing information to a real
// database (protocol, uri).
type createOrOverwriteDatabaseRequest struct {
	Name     string     `json:"name,omitempty"`
	Labels   []ui.Label `json:"labels,omitempty"`
	Protocol string     `json:"protocol,omitempty"`
	URI      string     `json:"uri,omitempty"`
	AWSRDS   *awsRDS    `json:"awsRds,omitempty"`
	// Overwrite will replace an existing db resource
	// with a new db resource. Only the name cannot
	// be changed.
	Overwrite bool `json:"overwrite,omitempty"`
}

type awsRDS struct {
	AccountID  string   `json:"accountId,omitempty"`
	ResourceID string   `json:"resourceId,omitempty"`
	Subnets    []string `json:"subnets,omitempty"`
	VPCID      string   `json:"vpcId,omitempty"`
}

func (r *createOrOverwriteDatabaseRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing database name")
	}

	if r.Protocol == "" {
		return trace.BadParameter("missing protocol")
	}

	if r.URI == "" {
		return trace.BadParameter("missing uri")
	}

	if r.AWSRDS != nil {
		if r.AWSRDS.ResourceID == "" {
			return trace.BadParameter("missing aws rds field resource id")
		}
		if r.AWSRDS.AccountID == "" {
			return trace.BadParameter("missing aws rds field account id")
		}
		if len(r.AWSRDS.Subnets) == 0 {
			return trace.BadParameter("missing aws rds field subnets")
		}
		if r.AWSRDS.VPCID == "" {
			return trace.BadParameter("missing aws rds field vpc id")
		}
	}

	return nil
}

// handleDatabaseCreate creates a database's metadata.
func (h *Handler) handleDatabaseCreateOrOverwrite(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var req *createOrOverwriteDatabaseRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := getNewDatabaseResource(*req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Overwrite {
		if _, err := clt.GetDatabase(r.Context(), req.Name); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := clt.UpdateDatabase(r.Context(), database); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := clt.CreateDatabase(r.Context(), database); err != nil {
			if trace.IsAlreadyExists(err) {
				return nil, trace.AlreadyExists("failed to create database (%q already exists), please use another name", req.Name)
			}
			return nil, trace.Wrap(err)
		}
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webui.MakeDatabase(database, accessChecker, h.cfg.DatabaseREPLRegistry, false /* requiresRequest */), nil
}

// updateDatabaseRequest contains some updatable fields of a database resource.
type updateDatabaseRequest struct {
	CACert *string    `json:"caCert,omitempty"`
	Labels []ui.Label `json:"labels,omitempty"`
	URI    string     `json:"uri,omitempty"`
	AWSRDS *awsRDS    `json:"awsRds,omitempty"`
}

func (r *updateDatabaseRequest) checkAndSetDefaults() error {
	if r.CACert != nil {
		if *r.CACert == "" {
			return trace.BadParameter("missing CA certificate data")
		}

		if _, err := tlsutils.ParseCertificatePEM([]byte(*r.CACert)); err != nil {
			return trace.BadParameter("could not parse provided CA as X.509 PEM certificate")
		}
	}

	// These fields can't be empty if set.
	if r.AWSRDS != nil {
		if r.AWSRDS.ResourceID == "" {
			return trace.BadParameter("missing aws rds field resource id")
		}
		if r.AWSRDS.AccountID == "" {
			return trace.BadParameter("missing aws rds field account id")
		}
	}

	if r.CACert == nil && r.AWSRDS == nil && r.Labels == nil && r.URI == "" {
		return trace.BadParameter("missing fields to update the database")
	}

	return nil
}

// handleDatabaseUpdate updates the database
func (h *Handler) handleDatabasePartialUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	databaseName := p.ByName("database")
	if databaseName == "" {
		return nil, trace.BadParameter("a database name is required")
	}

	var req *updateDatabaseRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database, err := clt.GetDatabase(r.Context(), databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	savedOrNewCaCert := database.GetCA()
	if req.CACert != nil {
		savedOrNewCaCert = *req.CACert
	}

	savedOrNewAWSRDS := awsRDS{
		AccountID:  database.GetAWS().AccountID,
		ResourceID: database.GetAWS().RDS.ResourceID,
	}
	if req.AWSRDS != nil {
		savedOrNewAWSRDS = awsRDS{
			AccountID:  req.AWSRDS.AccountID,
			ResourceID: req.AWSRDS.ResourceID,
		}
	}

	savedOrNewURI := req.URI
	if len(savedOrNewURI) == 0 {
		savedOrNewURI = database.GetURI()
	}

	savedLabels := database.GetStaticLabels()

	// Make a new database to reset the check and set defaulted fields.
	database, err = getNewDatabaseResource(createOrOverwriteDatabaseRequest{
		Name:     databaseName,
		Protocol: database.GetProtocol(),
		URI:      savedOrNewURI,
		Labels:   req.Labels,
		AWSRDS:   &savedOrNewAWSRDS,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	database.SetCA(savedOrNewCaCert)
	if len(req.Labels) == 0 {
		database.SetStaticLabels(savedLabels)
	}

	if err := clt.UpdateDatabase(r.Context(), database); err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return webui.MakeDatabase(database, accessChecker, h.cfg.DatabaseREPLRegistry, false /* requiresRequest */), nil
}

// databaseIAMPolicyResponse is the response type for handleDatabaseGetIAMPolicy.
type databaseIAMPolicyResponse struct {
	// Type is the type of the IAM policy.
	Type string `json:"type"`
	// AWS contains the IAM policy for AWS-hosted databases.
	AWS *databaseIAMPolicyAWS `json:"aws,omitempty"`
}

// databaseIAMPolicyAWS contains IAM policy for AWS-hosted databases.
type databaseIAMPolicyAWS struct {
	// PolicyDocument is the AWS IAM policy document.
	PolicyDocument string `json:"policy_document"`
	// Placeholders are placeholders found in the policy document.
	Placeholders []string `json:"placeholders,omitempty"`
}

// handleDatabaseGetIAMPolicy returns the required IAM policy for database.
func (h *Handler) handleDatabaseGetIAMPolicy(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	databaseName := p.ByName("database")
	if databaseName == "" {
		return nil, trace.BadParameter("missing database name")
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbServers, err := fetchDatabaseServersWithName(r.Context(), clt, r, databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	database := dbServers[0].GetDatabase()

	switch {
	case database.IsAWSHosted():
		policy, placeholders, err := dbiam.GetAWSPolicyDocument(database)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		policyJSON, err := json.Marshal(policy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &databaseIAMPolicyResponse{
			Type: "aws",
			AWS: &databaseIAMPolicyAWS{
				PolicyDocument: string(policyJSON),
				Placeholders:   placeholders,
			},
		}, nil

	default:
		return nil, trace.BadParameter("IAM policy not supported for database type %q", database.GetType())
	}
}

func (h *Handler) sqlServerConfigureADScriptHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	tokenStr := p.ByName("token")
	if err := validateJoinToken(tokenStr); err != nil {
		return "", trace.Wrap(err)
	}

	dbAddress := r.URL.Query().Get("uri")
	if err := services.ValidateSQLServerURI(dbAddress); err != nil {
		return "", trace.BadParameter("invalid database address: %v", err)
	}

	// verify that the token exists
	if _, err := h.GetProxyClient().GetToken(r.Context(), tokenStr); err != nil {
		return "", trace.BadParameter("invalid token")
	}

	proxyServers, err := h.GetProxyClient().GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(proxyServers) == 0 {
		return "", trace.NotFound("no proxy servers found")
	}

	clusterName, err := h.GetProxyClient().GetDomainName(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certAuthority, err := h.GetProxyClient().GetCertAuthority(
		r.Context(),
		types.CertAuthID{Type: types.DatabaseClientCA, DomainName: clusterName},
		false,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCRL, err := h.GetProxyClient().GenerateCertAuthorityCRL(r.Context(), types.DatabaseClientCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(certAuthority.GetActiveKeys().TLS) != 1 {
		return nil, trace.BadParameter("expected one TLS key pair, got %v", len(certAuthority.GetActiveKeys().TLS))
	}

	keyPair := certAuthority.GetActiveKeys().TLS[0]
	block, _ := pem.Decode(keyPair.Cert)
	if block == nil {
		return nil, trace.BadParameter("no PEM data in CA data")
	}

	// Split host and port so we can escape domain characters.
	dbHost, dbPort, err := net.SplitHostPort(dbAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httplib.SetScriptHeaders(w.Header())
	w.WriteHeader(http.StatusOK)
	err = scripts.DatabaseAccessSQLServerConfigureScript.Execute(w, scripts.DatabaseAccessSQLServerConfigureParams{
		CACertPEM:       string(keyPair.Cert),
		CACertSHA1:      fmt.Sprintf("%X", sha1.Sum(block.Bytes)),
		CACertBase64:    base64.StdEncoding.EncodeToString(utils.CreateCertificateBLOB(block.Bytes)),
		CRLPEM:          string(encodeCRLPEM(caCRL)),
		ProxyPublicAddr: proxyServers[0].GetPublicAddr(),
		ProvisionToken:  tokenStr,
		DBAddress:       net.JoinHostPort(url.QueryEscape(dbHost), dbPort),
	})

	return nil, trace.Wrap(err)
}

func (h *Handler) dbConnect(
	w http.ResponseWriter,
	r *http.Request,
	p httprouter.Params,
	sctx *SessionContext,
	site reversetunnelclient.RemoteSite,
	ws *websocket.Conn,
) (interface{}, error) {
	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	ctx, cancel := context.WithCancel(tctx)
	defer cancel()
	h.logger.DebugContext(ctx, "Received database interactive connection")

	req, err := readDatabaseSessionRequest(ws)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || terminal.IsOKWebsocketCloseError(trace.Unwrap(err)) {
			h.logger.DebugContext(ctx, "Database interactive session closed before receiving request")
			return nil, nil
		}

		var netError net.Error
		if errors.As(trace.Unwrap(err), &netError) && netError.Timeout() {
			return nil, trace.BadParameter("timed out waiting for database connect request data on websocket connection")
		}

		return nil, trace.Wrap(err)
	}

	log := h.logger.With(
		"protocol", req.Protocol,
		"service_name", req.ServiceName,
		"database_name", req.DatabaseName,
		"database_user", req.DatabaseUser,
		"database_roles", req.DatabaseRoles,
		"remote_addr", logutils.StringerAttr(ws.RemoteAddr()),
	)
	log.DebugContext(ctx, "Received database interactive session request")

	if !h.cfg.DatabaseREPLRegistry.IsSupported(req.Protocol) {
		log.ErrorContext(ctx, "Unsupported database protocol")
		return nil, trace.NotImplemented("%q database protocol not supported for REPL sessions", req.Protocol)
	}

	netConfig, err := h.GetAccessPoint().GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get host CA for this Proxy.
	clusterName, err := h.GetAccessPoint().GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyHostCA, err := h.GetAccessPoint().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := newDatabaseInteractiveSession(ctx, databaseInteractiveSessionConfig{
		log:               log,
		req:               req,
		ws:                ws,
		sctx:              sctx,
		site:              site,
		clt:               clt,
		keepAliveInterval: netConfig.GetKeepAliveInterval(),
		registry:          h.cfg.DatabaseREPLRegistry,
		alpnHandler:       h.cfg.ALPNHandler,
		proxyAddr:         h.PublicProxyAddr(),
		proxyHostCA:       proxyHostCA,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer sess.Close()

	// Don't close the terminal stream on session error, as it would also
	// cause the underlying connection to be closed. This will prevent the
	// middleware from properly writing the error into the WebSocket connection.
	if err := sess.Run(); err != nil {
		log.ErrorContext(ctx, "Database interactive session exited with error", "error", err)
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

// DatabaseSessionRequest describes a request to create a web-based terminal
// database session.
type DatabaseSessionRequest struct {
	// ServiceName is the database resource ID the user will be connected.
	ServiceName string `json:"serviceName"`
	// Protocol is the database protocol.
	Protocol string `json:"protocol"`
	// DatabaseName is the database name the session will use.
	DatabaseName string `json:"dbName"`
	// DatabaseUser is the database user used on the session.
	DatabaseUser string `json:"dbUser"`
	// DatabaseRoles are ratabase roles that will be attached to the user when
	// connecting to the database.
	DatabaseRoles []string `json:"dbRoles"`
}

// databaseConnectionRequestWaitTimeout defines how long the server will wait
// for the user to send the connection request.
const databaseConnectionRequestWaitTimeout = defaults.HeadlessLoginTimeout

// readDatabaseSessionRequest reads the database session requestion message from
// websocket connection.
func readDatabaseSessionRequest(ws *websocket.Conn) (*DatabaseSessionRequest, error) {
	err := ws.SetReadDeadline(time.Now().Add(databaseConnectionRequestWaitTimeout))
	if err != nil {
		return nil, trace.Wrap(err, "failed to set read deadline for websocket connection")
	}

	messageType, bytes, err := ws.ReadMessage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := ws.SetReadDeadline(time.Time{}); err != nil {
		return nil, trace.Wrap(err, "failed to set read deadline for websocket connection")
	}

	if messageType != websocket.BinaryMessage {
		return nil, trace.BadParameter("expected binary message of type websocket.BinaryMessage, got %v", messageType)
	}

	var envelope terminal.Envelope
	if err := gogoproto.Unmarshal(bytes, &envelope); err != nil {
		return nil, trace.BadParameter("failed to parse envelope: %v", err)
	}

	if envelope.Type != defaults.WebsocketDatabaseSessionRequest {
		return nil, trace.BadParameter("expected database session request but got %q", envelope.Type)
	}

	var req DatabaseSessionRequest
	if err := json.Unmarshal([]byte(envelope.Payload), &req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &req, nil
}

type databaseInteractiveSessionConfig struct {
	ws                *websocket.Conn
	log               *slog.Logger
	req               *DatabaseSessionRequest
	sctx              *SessionContext
	site              reversetunnelclient.RemoteSite
	clt               authclient.ClientI
	keepAliveInterval time.Duration
	registry          dbrepl.REPLRegistry
	alpnHandler       ConnectionHandler
	proxyAddr         string
	proxyHostCA       types.CertAuthority
}

func (c *databaseInteractiveSessionConfig) check() error {
	if c.ws == nil {
		return trace.BadParameter("missing parameter ws")
	}
	if c.req == nil {
		return trace.BadParameter("missing parameter req")
	}
	if c.site == nil {
		return trace.BadParameter("missing parameter site")
	}
	if c.clt == nil {
		return trace.BadParameter("missing parameter clt")
	}
	if c.keepAliveInterval == 0 {
		return trace.BadParameter("missing parameter keepAliveInterval")
	}
	if c.registry == nil {
		return trace.BadParameter("missing parameter registry")
	}
	if c.alpnHandler == nil {
		return trace.BadParameter("missing parameter alpnHandler")
	}
	if c.proxyAddr == "" {
		return trace.BadParameter("missing parameter proxyAddr")
	}
	if c.proxyHostCA == nil {
		return trace.BadParameter("missing parameter proxyHostCA")
	}
	return nil
}

type databaseInteractiveSession struct {
	databaseInteractiveSessionConfig
	ctx      context.Context
	replConn net.Conn
	alpnConn net.Conn
	stream   *terminal.Stream
}

func newDatabaseInteractiveSession(ctx context.Context, cfg databaseInteractiveSessionConfig) (*databaseInteractiveSession, error) {
	if err := cfg.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	replConn, alpnConn := net.Pipe()
	return &databaseInteractiveSession{
		ctx:                              ctx,
		databaseInteractiveSessionConfig: cfg,
		replConn:                         replConn,
		alpnConn:                         alpnConn,
		stream: terminal.NewStream(ctx, terminal.StreamConfig{
			WS: cfg.ws,
		}),
	}, nil
}

func (s *databaseInteractiveSession) Run() error {
	replConn, err := s.makeReplConn()
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.sendSessionMetadata(); err != nil {
		return trace.Wrap(err)
	}

	go startWSPingLoop(s.ctx, s.ws, s.keepAliveInterval, s.log, s.Close)

	// Wrap s.alpnConn with real client addresses and pass it to the ALPN
	// handler.
	go func() {
		alpnConnWithAddr := utils.NewConnWithAddr(s.alpnConn, s.ws.LocalAddr(), s.ws.RemoteAddr())
		if err := s.alpnHandler(s.ctx, alpnConnWithAddr); !utils.IsOKNetworkError(err) {
			s.log.ErrorContext(s.ctx, "ALPN handler for database interactive session failed", "error", err)
		}
	}()

	repl, err := s.registry.NewInstance(s.ctx, &dbrepl.NewREPLConfig{
		Client:     s.stream,
		ServerConn: replConn,
		Route:      s.route(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.DebugContext(s.ctx, "Starting database interactive session")
	if err := repl.Run(s.ctx); err != nil {
		return trace.Wrap(err)
	}

	s.log.DebugContext(s.ctx, "Database interactive session exited with success")
	return nil
}

func (s *databaseInteractiveSession) Close() error {
	// TODO(gabrielcorado): Right now, if we send a close message the UI closes
	// the terminal without giving the chance for users to review the session.
	// Once this gets solved, we should send the close message here.
	if err := s.stream.Close(); err != nil {
		s.log.ErrorContext(s.ctx, "Unable to close web socket terminal stream", "error", err)
	}

	if err := s.replConn.Close(); !utils.IsOKNetworkError(err) {
		return trace.Wrap(err)
	}
	return nil
}

// issueCerts performs the MFA (if required) and generate the user session
// certificates.
func (s *databaseInteractiveSession) issueCerts() (*tls.Certificate, error) {
	pk, err := keys.ParsePrivateKey(s.sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed getting user private key from the session")
	}

	publicKeyPEM, err := keys.MarshalPublicKey(pk.Public())
	if err != nil {
		return nil, trace.Wrap(err, "failed to marshal public key")
	}

	routeToDatabase := s.route()

	certsReq := proto.UserCertsRequest{
		TLSPublicKey:    publicKeyPEM,
		Username:        s.sctx.GetUser(),
		Expires:         s.sctx.cfg.Session.GetExpiryTime(),
		Format:          constants.CertificateFormatStandard,
		RouteToCluster:  s.site.GetName(),
		Usage:           proto.UserCertsRequest_Database,
		RouteToDatabase: routeToDatabase,
	}

	var certs *proto.Certs
	result, err := client.PerformSessionMFACeremony(s.ctx, client.PerformSessionMFACeremonyParams{
		CurrentAuthClient: s.clt,
		RootAuthClient:    s.sctx.cfg.RootClient,
		MFACeremony:       newMFACeremony(s.stream.WSStream, s.sctx.cfg.RootClient.CreateAuthenticateChallenge, s.proxyAddr),
		MFAAgainstRoot:    s.sctx.cfg.RootClusterName == s.site.GetName(),
		MFARequiredReq: &proto.IsMFARequiredRequest{
			Target: &proto.IsMFARequiredRequest_Database{Database: &routeToDatabase},
		},
		CertsReq: &certsReq,
	})
	if err != nil && !errors.Is(err, services.ErrSessionMFANotRequired) {
		return nil, trace.Wrap(err, "failed performing mfa ceremony")
	}

	if result != nil {
		certs = result.NewCerts
	}

	if certs == nil {
		certs, err = s.sctx.cfg.RootClient.GenerateUserCerts(s.ctx, certsReq)
		if err != nil {
			return nil, trace.Wrap(err, "failed issuing user certs")
		}
	}

	tlsCert, err := pk.TLSCertificate(certs.TLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &tlsCert, nil
}

// makeReplConn wraps the raw repl conn with a TLS certificate to simulate a
// dialed TLS routing connection.
func (s *databaseInteractiveSession) makeReplConn() (*tls.Conn, error) {
	tlsCert, err := s.issueCerts()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	alpnProtocol, err := alpncommon.ToALPNProtocol(s.req.Protocol)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyAddr, err := utils.ParseAddr(s.proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The ALPN handler used by the web server was initially intended for ALPN
	// connection upgrade. Database handlers serve with the Proxy's host cert on
	// the other side.
	rootCAs, err := services.CertPool(s.proxyHostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig := &tls.Config{
		NextProtos:   []string{string(alpnProtocol)},
		Certificates: []tls.Certificate{*tlsCert},
		RootCAs:      rootCAs,
		ServerName:   proxyAddr.Host(),
	}
	utils.SetupTLSConfig(tlsConfig, nil /* let server decide cipher */)

	return tls.Client(s.replConn, tlsConfig), nil
}

func (s *databaseInteractiveSession) route() proto.RouteToDatabase {
	return proto.RouteToDatabase{
		Protocol:    s.req.Protocol,
		ServiceName: s.req.ServiceName,
		Username:    s.req.DatabaseUser,
		Database:    s.req.DatabaseName,
		Roles:       s.req.DatabaseRoles,
	}
}

func (s *databaseInteractiveSession) sendSessionMetadata() error {
	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: session.Session{
		// TODO(gabrielcorado): Have a consistent Session ID. Right now, the
		// initial session ID returned won't be correct as the session is only
		// initialized by the database server after the REPL starts.
		ClusterName: s.site.GetName(),
	}})
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := gogoproto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// fetchDatabaseServersWithName fetches all database servers with provided database name.
func fetchDatabaseServersWithName(ctx context.Context, clt resourcesAPIGetter, r *http.Request, databaseName string) ([]types.DatabaseServer, error) {
	resp, err := clt.ListResources(ctx, proto.ListResourcesRequest{
		Limit:               defaults.MaxIterationLimit,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, databaseName),
		UseSearchAsRoles:    r.URL.Query().Get("searchAsRoles") == "yes",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(resp.Resources).AsDatabaseServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(servers) == 0 {
		return nil, trace.NotFound("database %q not found", databaseName)
	}
	return servers, nil
}

func getNewDatabaseResource(req createOrOverwriteDatabaseRequest) (*types.DatabaseV3, error) {
	labels := make(map[string]string)
	for _, label := range req.Labels {
		labels[label.Name] = label.Value
	}

	dbSpec := types.DatabaseSpecV3{
		Protocol: req.Protocol,
		URI:      req.URI,
	}

	if req.AWSRDS != nil {
		dbSpec.AWS = types.AWS{
			AccountID: req.AWSRDS.AccountID,
			RDS: types.RDS{
				ResourceID: req.AWSRDS.ResourceID,
				Subnets:    req.AWSRDS.Subnets,
				VPCID:      req.AWSRDS.VPCID,
			},
		}
	}

	database, err := types.NewDatabaseV3(
		types.Metadata{
			Name:   req.Name,
			Labels: labels,
		}, dbSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	database.SetOrigin(types.OriginDynamic)

	return database, nil
}

// encodeCRLPEM takes DER encoded CRL and encodes into PEM.
func encodeCRLPEM(contents []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "X509 CRL",
		Bytes: contents,
	})
}
