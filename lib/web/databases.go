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
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/web/scripts"
	"github.com/gravitational/teleport/lib/web/ui"
)

// createDatabaseRequest contains the necessary basic information to create a database.
// Database here is the database resource, containing information to a real database (protocol, uri)
type createDatabaseRequest struct {
	Name     string     `json:"name,omitempty"`
	Labels   []ui.Label `json:"labels,omitempty"`
	Protocol string     `json:"protocol,omitempty"`
	URI      string     `json:"uri,omitempty"`
	AWSRDS   *awsRDS    `json:"awsRds,omitempty"`
}

type awsRDS struct {
	AccountID  string   `json:"accountId,omitempty"`
	ResourceID string   `json:"resourceId,omitempty"`
	Subnets    []string `json:"subnets,omitempty"`
	VPCID      string   `json:"vpcId,omitempty"`
}

func (r *createDatabaseRequest) checkAndSetDefaults() error {
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
func (h *Handler) handleDatabaseCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var req *createDatabaseRequest
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

	if err := clt.CreateDatabase(r.Context(), database); err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("failed to create database (%q already exists), please use another name", req.Name)
		}
		return nil, trace.Wrap(err)
	}

	accessChecker, err := sctx.GetUserAccessChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbNames, dbUsers, err := getDatabaseUsersAndNames(accessChecker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.MakeDatabase(database, dbUsers, dbNames), nil
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
func (h *Handler) handleDatabaseUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
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
	database, err = getNewDatabaseResource(createDatabaseRequest{
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

	return ui.MakeDatabase(database, nil /* dbUsers */, nil /* dbNames */), nil
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

	certAuthority, err := getCAForSQLServerConfigureADScript(r.Context(), h.GetProxyClient())
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
		CACertBase64:    base64.StdEncoding.EncodeToString(createCertificateBlob(block.Bytes)),
		CRLPEM:          string(encodeCRLPEM(caCRL)),
		ProxyPublicAddr: proxyServers[0].GetPublicAddr(),
		ProvisionToken:  tokenStr,
		DBAddress:       net.JoinHostPort(url.QueryEscape(dbHost), dbPort),
	})

	return nil, trace.Wrap(err)
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

func getNewDatabaseResource(req createDatabaseRequest) (*types.DatabaseV3, error) {
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

// getCAForSQLServerConfigureADScript is a helper for sql server configuration
// that fetches the DatabaseClientCA if the auth service supports it or falls back
// to the DatabaseCA if auth service does not support it.
// TODO(gavin): DELETE IN 16.0.0
func getCAForSQLServerConfigureADScript(ctx context.Context, clusterAPI authclient.ClientI) (types.CertAuthority, error) {
	domainName, err := clusterAPI.GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dbClientCA, err := clusterAPI.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseClientCA,
		DomainName: domainName,
	}, false)
	if err == nil {
		return dbClientCA, nil
	}
	if !types.IsUnsupportedAuthorityErr(err) {
		return nil, trace.Wrap(err)
	}

	// fallback to DatabaseCA if DatabaseClientCA isn't supported by backend.
	dbServerCA, err := clusterAPI.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.DatabaseCA,
		DomainName: domainName,
	}, false)
	return dbServerCA, trace.Wrap(err)
}
