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
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/client/proto"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	postgresrepl "github.com/gravitational/teleport/lib/client/db/repl/postgres"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web/scripts"
	"github.com/gravitational/teleport/lib/web/terminal"
	"github.com/gravitational/teleport/lib/web/ui"
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
	if err := httplib.ReadJSON(r, &req); err != nil {
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
	dbNames, dbUsers, err := getDatabaseUsersAndNames(accessChecker)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbRoles, err := getDatabaseRolesNames(accessChecker, database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDatabase(database, dbUsers, dbRoles, dbNames, false /* requiresRequest */), nil
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
	if err := httplib.ReadJSON(r, &req); err != nil {
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

	return ui.MakeDatabase(database, nil /* dbUsers */, nil /* dbRoles */, nil /* dbNames */, false /* requiresRequest */), nil
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

	database, err := fetchDatabaseWithName(r.Context(), clt, r, databaseName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
	fmt.Println("=====>>>>> STARTED DATABASE WEBSOCKET REQUEST")
	log := h.log.WithField(teleport.ComponentKey, "db")
	req, err := readDatabaseSessionRequest(ws)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || terminal.IsOKWebsocketCloseError(trace.Unwrap(err)) {
			return nil, nil
		}

		var netError net.Error
		if errors.As(trace.Unwrap(err), &netError) && netError.Timeout() {
			return nil, trace.BadParameter("timed out waiting for pod exec request data on websocket connection")
		}

		return nil, trace.Wrap(err)
	}

	fmt.Println("=====>>>>> RECEIVED DATABASE CONNECT REQUEST", req)

	// TODO: should we validate the request?

	// ====>>> SETUP WS TERMINAL
	// Create a context for signaling when the terminal session is over and
	// link it first with the trace context from the request context
	tctx := oteltrace.ContextWithRemoteSpanContext(context.Background(), oteltrace.SpanContextFromContext(r.Context()))
	ctx, cancel := context.WithCancel(tctx)
	defer cancel()

	// TODO: do we need to implement any handler?
	stream := terminal.NewStream(ctx, terminal.StreamConfig{WS: ws, Logger: log})
	// ====>>> END SETUP WS TERMINAL

	// ====>>> CERTIFICATE GENERATION
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// GenerateRSAKeyRing
	pk, err := keys.ParsePrivateKey(sctx.cfg.Session.GetTLSPriv())
	if err != nil {
		return nil, trace.Wrap(err, "failed getting user private key from the session")
	}

	// privateKeyPEM, err := pk.SoftwarePrivateKeyPEM()
	// if err != nil {
	// 	return nil, trace.Wrap(err, "failed getting software private key")
	// }
	publicKeyPEM, err := keys.MarshalPublicKey(pk.Public())
	if err != nil {
		return nil, trace.Wrap(err, "failed to marshal public key")
	}

	// TODO: Should we validate database exists here?
	routeToDatabase := clientproto.RouteToDatabase{
		// TODO: bring this from database.
		Protocol:    defaults.ProtocolPostgres,
		ServiceName: req.ServiceName,
		Username:    req.DatabaseUser,
		Database:    req.DatabaseName,
		// TODO: support roles checking.
		// Roles: []string{},
	}
	certsReq := clientproto.UserCertsRequest{
		TLSPublicKey:    publicKeyPEM,
		Username:        sctx.GetUser(),
		Expires:         sctx.cfg.Session.GetExpiryTime(),
		Format:          constants.CertificateFormatStandard,
		RouteToCluster:  site.GetName(),
		Usage:           clientproto.UserCertsRequest_Database,
		RouteToDatabase: routeToDatabase,
	}

	_, certs, err := client.PerformMFACeremony(ctx, client.PerformMFACeremonyParams{
		CurrentAuthClient: clt,
		RootAuthClient:    sctx.cfg.RootClient,
		MFAPrompt: mfa.PromptFunc(func(ctx context.Context, chal *clientproto.MFAAuthenticateChallenge) (*clientproto.MFAAuthenticateResponse, error) {
			assertion, err := promptMFAChallenge(stream.WSStream, protobufMFACodec{}).Run(ctx, chal)
			return assertion, trace.Wrap(err)
		}),
		MFAAgainstRoot: sctx.cfg.RootClusterName == site.GetName(),
		MFARequiredReq: &clientproto.IsMFARequiredRequest{
			Target: &clientproto.IsMFARequiredRequest_Database{Database: &routeToDatabase},
		},
		ChallengeExtensions: mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
		},
		CertsReq: &certsReq,
	})
	if err != nil && !errors.Is(err, services.ErrSessionMFANotRequired) {
		return nil, trace.Wrap(err, "failed performing mfa ceremony")
	}

	if certs == nil {
		certs, err = sctx.cfg.RootClient.GenerateUserCerts(ctx, certsReq)
		if err != nil {
			return nil, trace.Wrap(err, "failed issuing user certs")
		}
	}
	// ====>>> END CERTIFICATE GENERATION

	// ====>>> CONNECT TO DATABASE SERVER

	accessPoint, err := site.CachingAccessPoint()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := accessPoint.GetDatabaseServers(ctx, apidefaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result []types.DatabaseServer
	for _, server := range servers {
		if server.GetDatabase().GetName() == routeToDatabase.ServiceName {
			result = append(result, server)
		}
	}
	if len(result) == 0 {
		return nil, trace.NotFound("database %q not found among registered databases in cluster %q",
			routeToDatabase.ServiceName,
			site.GetName())
	}

	parsedCert, err := tlsca.ParseCertificatePEM(certs.TLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: do we need to go back and forth?
	identity, err := tlsca.FromSubject(parsedCert.Subject, parsedCert.NotAfter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sid := session.NewID()
	identity.RouteToDatabase.SessionID = sid.String()

	server := result[0]
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := generateDatabaseServerTLSConfig(ctx, site.GetName(), subject, accessPoint, h.cfg.ProxyClient, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceConn, err := site.Dial(reversetunnelclient.DialParams{
		// TODO: do we need those?
		// From:                  r.RemoteAddr(),
		// OriginalClientDstAddr: clientDstAddr,
		To:       &utils.NetAddr{AddrNetwork: "tcp", Addr: reversetunnelclient.LocalNode},
		ServerID: fmt.Sprintf("%v.%v", server.GetHostID(), site.GetName()),
		ConnType: types.DatabaseTunnel,
		ProxyIDs: server.GetProxyIDs(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Upgrade the connection so the client identity can be passed to the
	// remote server during TLS handshake. On the remote side, the connection
	// received from the reverse tunnel will be handled by tls.Server.
	serviceConn = tls.Client(serviceConn, tlsConfig)

	// ====>>> END CONNECT TO DATABASE SERVER

	// tlsCert, err := keys.X509KeyPair(certs.TLS, privateKeyPEM)
	// if err != nil {
	// 	return tls.Certificate{}, trace.BadParameter("failed to parse private key: %v", err)
	// }

	sessionMetadataResponse, err := json.Marshal(siteSessionGenerateResponse{Session: session.Session{
		ID:          sid,
		ClusterName: site.GetName(),
	}})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envelope := &terminal.Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketSessionMetadata,
		Payload: string(sessionMetadataResponse),
	}

	envelopeBytes, err := gogoproto.Marshal(envelope)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = ws.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: generate the REPL thing.
	repl, err := postgresrepl.New(ctx, stream, serviceConn, routeToDatabase)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer repl.Close()

	if err := repl.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	return nil, nil
}

func generateDatabaseServerTLSConfig(ctx context.Context, clusterName string, subject pkix.Name, accessPoint authclient.RemoteProxyAccessPoint, authClient authclient.ClientI, server types.DatabaseServer) (*tls.Config, error) {
	privateKey, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(accessPoint),
		cryptosuites.ProxyToDatabaseAgent)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := authClient.SignDatabaseCSR(ctx, &proto.DatabaseCSRRequest{
		CSR:         csr,
		ClusterName: clusterName,
	})
	if err != nil {
		fmt.Println("=====>>>> SIGNDATABSE CSR ERR", err)
		return nil, trace.Wrap(err)
	}

	cert, err := keys.TLSCertificateForSigner(privateKey, response.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pool := x509.NewCertPool()
	for _, caCert := range response.CACerts {
		ok := pool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, trace.BadParameter("failed to append CA certificate")
		}
	}

	return &tls.Config{
		ServerName:   server.GetHostname(),
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// DatabaseSessionRequest describes a request to create a web-based terminal
// database session.
type DatabaseSessionRequest struct {
	// ServiceName is the database resource ID the user will be connected.
	ServiceName string `json:"serviceName"`
	// DatabaseName is the database name the session will use.
	DatabaseName string `json:"dbName"`
	// DatabaseUser is the database user used on the session.
	DatabaseUser string `json:"dbUser"`
	// DatabaseRoles are ratabase roles that will be attached to the user when
	// connecting to the database.
	DatabaseRoles string `json:"dbRoles"`
}

const databaseConnectionRequestWaitTimeout = defaults.HeadlessLoginTimeout

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
		return nil, trace.BadParameter("Expected binary message of type websocket.BinaryMessage, got %v", messageType)
	}

	var envelope terminal.Envelope
	if err := gogoproto.Unmarshal(bytes, &envelope); err != nil {
		return nil, trace.BadParameter("Failed to parse envelope: %v", err)
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

// fetchDatabaseWithName fetch a database with provided database name.
func fetchDatabaseWithName(ctx context.Context, clt resourcesAPIGetter, r *http.Request, databaseName string) (types.Database, error) {
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

	switch len(servers) {
	case 0:
		return nil, trace.NotFound("database %q not found", databaseName)
	default:
		return servers[0].GetDatabase(), nil
	}
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
