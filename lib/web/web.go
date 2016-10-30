/*
Copyright 2015 Gravitational, Inc.

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

// Package web implements web proxy handler that provides
// web interface to view and connect to teleport nodes
package web

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/mailgun/lemma/secret"
	"github.com/mailgun/ttlmap"
)

// Handler is HTTP web proxy handler
type Handler struct {
	sync.Mutex
	httprouter.Router
	cfg                     Config
	auth                    *sessionCache
	sites                   *ttlmap.TtlMap
	sessionStreamPollPeriod time.Duration
}

// HandlerOption is a functional argument - an option that can be passed
// to NewHandler function
type HandlerOption func(h *Handler) error

// SetSessionStreamPollPeriod sets polling period for session streams
func SetSessionStreamPollPeriod(period time.Duration) HandlerOption {
	return func(h *Handler) error {
		if period < 0 {
			return trace.BadParameter("period should be non zero")
		}
		h.sessionStreamPollPeriod = period
		return nil
	}
}

// Config represents web handler configuration parameters
type Config struct {
	// Proxy is a reverse tunnel proxy that handles connections
	// to various sites
	Proxy reversetunnel.Server
	// AuthServers is a list of auth servers this proxy talks to
	AuthServers utils.NetAddr
	// DomainName is a domain name served by web handler
	DomainName string
	// ProxyClient is a client that authenticated as proxy
	ProxyClient auth.ClientI
	// DisableUI allows to turn off serving web based UI
	DisableUI bool
}

// Version is a current webapi version
const APIVersion = "v1"

// NewHandler returns a new instance of web proxy handler
func NewHandler(cfg Config, opts ...HandlerOption) (*Handler, error) {
	const apiPrefix = "/" + APIVersion
	lauth, err := newSessionCache([]utils.NetAddr{cfg.AuthServers})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		cfg:  cfg,
		auth: lauth,
	}

	for _, o := range opts {
		if err := o(h); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if h.sessionStreamPollPeriod == 0 {
		h.sessionStreamPollPeriod = sessionStreamPollPeriod
	}

	// Web sessions
	h.POST("/webapi/sessions", httplib.MakeHandler(h.createSession))
	h.DELETE("/webapi/sessions", h.withAuth(h.deleteSession))
	h.POST("/webapi/sessions/renew", h.withAuth(h.renewSession))

	// Users
	h.GET("/webapi/users/invites/:token", httplib.MakeHandler(h.renderUserInvite))
	h.POST("/webapi/users", httplib.MakeHandler(h.createNewUser))

	// Issues SSH temp certificates based on 2FA access creds
	h.POST("/webapi/ssh/certs", httplib.MakeHandler(h.createSSHCert))

	// list available sites
	h.GET("/webapi/sites", h.withAuth(h.getSites))

	// Site specific API

	// get nodes
	h.GET("/webapi/sites/:site/nodes", h.withSiteAuth(h.getSiteNodes))
	// connect to node via websocket (that's why it's a GET method)
	h.GET("/webapi/sites/:site/connect", h.withSiteAuth(h.siteNodeConnect))
	// get session event stream
	h.GET("/webapi/sites/:site/sessions/:sid/events/stream", h.withSiteAuth(h.siteSessionStream))
	// generate a new session
	h.POST("/webapi/sites/:site/sessions", h.withSiteAuth(h.siteSessionGenerate))
	// update session parameters
	h.PUT("/webapi/sites/:site/sessions/:sid", h.withSiteAuth(h.siteSessionUpdate))
	// get the session list
	h.GET("/webapi/sites/:site/sessions", h.withSiteAuth(h.siteSessionsGet))
	// get a session
	h.GET("/webapi/sites/:site/sessions/:sid", h.withSiteAuth(h.siteSessionGet))
	// get session's events
	h.GET("/webapi/sites/:site/sessions/:sid/events", h.withSiteAuth(h.siteSessionEventsGet))
	// get session's bytestream
	h.GET("/webapi/sites/:site/sessions/:sid/stream", h.siteSessionStreamGet)
	// search site events
	h.GET("/webapi/sites/:site/events", h.withSiteAuth(h.siteEventsGet))

	// OIDC related callback handlers
	h.GET("/webapi/oidc/login/web", httplib.MakeHandler(h.oidcLoginWeb))
	h.POST("/webapi/oidc/login/console", httplib.MakeHandler(h.oidcLoginConsole))
	h.GET("/webapi/oidc/callback", httplib.MakeHandler(h.oidcCallback))

	// if Web UI is enabled, chekc the assets dir:
	var (
		writeSettings http.HandlerFunc
		indexPage     *template.Template
		staticFS      http.FileSystem
	)
	if !cfg.DisableUI {
		staticFS, err = NewStaticFileSystem()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		index, err := staticFS.Open("/index.html")
		if err != nil {
			log.Error(err)
			return nil, trace.Wrap(err)
		}
		defer index.Close()
		indexContent, err := ioutil.ReadAll(index)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		indexPage, err = template.New("index").Parse(string(indexContent))
		if err != nil {
			return nil, trace.BadParameter("failed parsing index.html template: %v", err)
		}
		writeSettings = httplib.MakeStdHandler(h.getSettings)
	}

	routingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// request is going to the API?
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			http.StripPrefix(apiPrefix, h).ServeHTTP(w, r)
			return
		}

		// request is going to the web UI
		if cfg.DisableUI {
			w.WriteHeader(http.StatusNotImplemented)
			return
		}

		// redirect to "/web" when someone hits "/"
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/web", http.StatusFound)
			return
		}

		// serve Web UI:
		if strings.HasPrefix(r.URL.Path, "/web/app") {
			http.StripPrefix("/web", http.FileServer(staticFS)).ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web/config.js") {
			writeSettings.ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web") {
			ctx, err := h.AuthenticateRequest(w, r, false)
			session := struct {
				Session string
			}{Session: base64.StdEncoding.EncodeToString([]byte("{}"))}
			if err == nil {
				re, err := NewSessionResponse(ctx)
				if err == nil {
					out, err := json.Marshal(re)
					if err == nil {
						session.Session = base64.StdEncoding.EncodeToString(out)
					}
				}
			}
			indexPage.Execute(w, session)
		} else {
			http.NotFound(w, r)
		}
	})
	h.NotFound = routingHandler
	return h, nil
}

// Close closes associated session cache operations
func (m *Handler) Close() error {
	return m.auth.Close()
}

type webSettings struct {
	Auth struct {
		OIDCConnectors []string `json:"oidc_connectors"`
	} `json:"auth"`
}

func (m *Handler) getSettings(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	settings := &webSettings{}
	connectors, err := m.cfg.ProxyClient.GetOIDCConnectors(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, connector := range connectors {
		settings.Auth.OIDCConnectors = append(settings.Auth.OIDCConnectors, connector.ID)
	}
	if len(settings.Auth.OIDCConnectors) == 0 {
		settings.Auth.OIDCConnectors = make([]string, 0)
	}
	out, err := json.Marshal(settings)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fmt.Fprintf(w, "var GRV_CONFIG = %v;", string(out))
	return nil, nil
}

func (m *Handler) oidcLoginWeb(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	log.Infof("oidcLoginWeb start")
	query := r.URL.Query()
	clientRedirectURL := query.Get("redirect_url")
	if clientRedirectURL == "" {
		return nil, trace.BadParameter("missing redirect_url query parameter")
	}
	connectorID := query.Get("connector_id")
	if connectorID == "" {
		return nil, trace.BadParameter("missing connector_id query parameter")
	}
	response, err := m.cfg.ProxyClient.CreateOIDCAuthRequest(
		services.OIDCAuthRequest{
			ConnectorID:       connectorID,
			CreateWebSession:  true,
			ClientRedirectURL: clientRedirectURL,
			CheckUser:         true,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	http.Redirect(w, r, response.RedirectURL, http.StatusFound)
	return nil, nil
}

type oidcLoginConsoleReq struct {
	RedirectURL string        `json:"redirect_url"`
	PublicKey   []byte        `json:"public_key"`
	CertTTL     time.Duration `json:"cert_ttl"`
	ConnectorID string        `json:"connector_id"`
}

type oidcLoginConsoleResponse struct {
	RedirectURL string `json:"redirect_url"`
}

func (m *Handler) oidcLoginConsole(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	log.Infof("oidcLoginConsole start")
	var req *oidcLoginConsoleReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	if req.RedirectURL == "" {
		return nil, trace.BadParameter("missing RedirectURL")
	}
	if len(req.PublicKey) == 0 {
		return nil, trace.BadParameter("missing PublicKey")
	}
	if req.ConnectorID == "" {
		return nil, trace.BadParameter("missing ConnectorID")
	}
	response, err := m.cfg.ProxyClient.CreateOIDCAuthRequest(
		services.OIDCAuthRequest{
			ConnectorID:       req.ConnectorID,
			ClientRedirectURL: req.RedirectURL,
			PublicKey:         req.PublicKey,
			CertTTL:           req.CertTTL,
			CheckUser:         true,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &oidcLoginConsoleResponse{RedirectURL: response.RedirectURL}, nil
}

func (m *Handler) oidcCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	log.Infof("oidcCallback start")
	response, err := m.cfg.ProxyClient.ValidateOIDCAuthCallback(r.URL.Query())
	if err != nil {
		log.Infof("VALIDATE error: %v", err)
		return nil, trace.Wrap(err)
	}
	// if we created web session, set session cookie and redirect to original url
	if response.Req.CreateWebSession {
		log.Infof("oidcCallback redirecting to web browser")
		if err := SetSession(w, response.Username, response.Session.ID); err != nil {
			return nil, trace.Wrap(err)
		}
		http.Redirect(w, r, response.Req.ClientRedirectURL, http.StatusFound)
		return nil, nil
	}
	log.Infof("oidcCallback redirecting to console login")
	if len(response.Req.PublicKey) == 0 {
		return nil, trace.BadParameter("not a web or console oidc login request")
	}
	redirectURL, err := ConstructSSHResponse(response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	return nil, nil
}

// ConstructSSHResponse creates a special SSH response for SSH login method
// that encodes everything using the client's secret key
func ConstructSSHResponse(response *auth.OIDCAuthResponse) (*url.URL, error) {
	u, err := url.Parse(response.Req.ClientRedirectURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	consoleResponse := SSHLoginResponse{
		Username:    response.Username,
		Cert:        response.Cert,
		HostSigners: response.HostSigners,
	}
	out, err := json.Marshal(consoleResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := u.Query()
	secretKey := values.Get("secret")
	if secretKey == "" {
		return nil, trace.BadParameter("missing secret")
	}
	values.Set("secret", "") // remove secret so others can't see it
	secretKeyBytes, err := secret.EncodedStringToKey(secretKey)
	if err != nil {
		return nil, trace.BadParameter("bad secret")
	}
	encryptor, err := secret.New(&secret.Config{KeyBytes: secretKeyBytes})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sealedBytes, err := encryptor.Seal(out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sealedBytesData, err := json.Marshal(sealedBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values.Set("response", string(sealedBytesData))
	u.RawQuery = values.Encode()
	return u, nil
}

// createSessionReq is a request to create session from username, password and second
// factor token
type createSessionReq struct {
	User              string `json:"user"`
	Pass              string `json:"pass"`
	SecondFactorToken string `json:"second_factor_token"`
}

// CreateSessionResponse returns OAuth compabible data about
// access token: https://tools.ietf.org/html/rfc6749
type CreateSessionResponse struct {
	// Type is token type (bearer)
	Type string `json:"type"`
	// Token value
	Token string `json:"token"`
	// User represents the user
	User services.User `json:"user"`
	// ExpiresIn sets seconds before this token is not valid
	ExpiresIn int `json:"expires_in"`
}

type createSessionResponseRaw struct {
	// Type is token type (bearer)
	Type string `json:"type"`
	// Token value
	Token string `json:"token"`
	// User represents the user
	User json.RawMessage `json:"user"`
	// ExpiresIn sets seconds before this token is not valid
	ExpiresIn int `json:"expires_in"`
}

func (r createSessionResponseRaw) response() (*CreateSessionResponse, error) {
	user, err := services.GetUserUnmarshaler()(r.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CreateSessionResponse{Type: r.Type, Token: r.Token, ExpiresIn: r.ExpiresIn, User: user.WebSessionInfo()}, nil
}

func NewSessionResponse(ctx *SessionContext) (*CreateSessionResponse, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	webSession := ctx.GetWebSession()
	user, err := clt.GetUser(webSession.Username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CreateSessionResponse{
		Type:      roundtrip.AuthBearer,
		Token:     webSession.WS.BearerToken,
		User:      user.WebSessionInfo(),
		ExpiresIn: int(time.Now().Sub(webSession.WS.Expires) / time.Second),
	}, nil
}

// createSession creates a new web session based on user, pass and 2nd factor token
//
// POST /v1/webapi/sessions
//
// {"user": "alex", "pass": "abc123", "second_factor_token": "token"}
//
// Response
//
// {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20}
//
func (m *Handler) createSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := m.auth.Auth(req.User, req.Pass, req.SecondFactorToken)
	if err != nil {
		log.Infof("bad access credentials: %v", err)
		return nil, trace.AccessDenied("bad auth credentials")
	}
	if err := SetSession(w, req.User, sess.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, err := m.auth.ValidateSession(req.User, sess.ID)
	if err != nil {
		return nil, trace.AccessDenied("need auth")
	}
	return NewSessionResponse(ctx)
}

// deleteSession is called to sign out user
//
// DELETE /v1/webapi/sessions/:sid
//
// Response:
//
// {"message": "ok"}
//
func (m *Handler) deleteSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	if err := ctx.Invalidate(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ClearSession(w); err != nil {
		return nil, trace.Wrap(err)
	}
	return ok(), nil
}

// renewSession is called to renew the session that is about to expire
// it issues the new session and generates new session cookie.
// It's important to understand that the old session becomes effectively invalid.
//
// POST /v1/webapi/sessions/renew
//
// Response
//
// {"type": "bearer", "token": "bearer token", "user": {"name": "alex", "allowed_logins": ["admin", "bob"]}, "expires_in": 20}
//
//
func (m *Handler) renewSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx *SessionContext) (interface{}, error) {
	newSess, err := ctx.ExtendWebSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// transfer ownership over connections that were opened in the
	// sessionContext
	newContext, err := ctx.parent.ValidateSession(newSess.Username, newSess.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	newContext.AddClosers(ctx.TransferClosers()...)
	if err := SetSession(w, newSess.Username, newSess.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewSessionResponse(newContext)
}

type renderUserInviteResponse struct {
	InviteToken string `json:"invite_token"`
	User        string `json:"user"`
	QR          []byte `json:"qr"`
}

// renderUserInvite is called to show user the new user invitation page
//
// GET /v1/webapi/users/invites/:token
//
// Response:
//
// {"invite_token": "token", "user": "alex", qr: "base64-encoded-qr-code image"}
//
//
func (m *Handler) renderUserInvite(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	token := p[0].Value
	user, QRCodeBytes, _, err := m.auth.GetUserInviteInfo(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &renderUserInviteResponse{
		InviteToken: token,
		User:        user,
		QR:          QRCodeBytes,
	}, nil
}

// createNewUser req is a request to create a new Teleport user
type createNewUserReq struct {
	InviteToken       string `json:"invite_token"`
	Pass              string `json:"pass"`
	SecondFactorToken string `json:"second_factor_token"`
}

// createNewUser creates new user entry based on the invite token
//
// POST /v1/webapi/users
//
// {"invite_token": "unique invite token", "pass": "user password", "second_factor_token": "valid second factor token"}
//
// Sucessful response: (session cookie is set)
//
// {"type": "bearer", "token": "bearer token", "user": "alex", "expires_in": 20}
func (m *Handler) createNewUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createNewUserReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := m.auth.CreateNewUser(req.InviteToken, req.Pass, req.SecondFactorToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, err := m.auth.ValidateSession(sess.Username, sess.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := SetSession(w, sess.Username, sess.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	return NewSessionResponse(ctx)
}

type getSitesResponse struct {
	Sites []site `json:"sites"`
}

type site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"last_connected"`
	Status        string    `json:"status"`
}

func convertSites(rs []reversetunnel.RemoteSite) []site {
	out := make([]site, len(rs))
	for i := range rs {
		out[i] = site{
			Name:          rs[i].GetName(),
			LastConnected: rs[i].GetLastConnected(),
			Status:        rs[i].GetStatus(),
		}
	}
	return out
}

// getSites returns a list of sites
//
// GET /v1/webapi/sites
//
// Sucessful response:
//
// {"sites": {"name": "localhost", "last_connected": "RFC3339 time", "status": "active"}}
//
func (m *Handler) getSites(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *SessionContext) (interface{}, error) {
	return getSitesResponse{
		Sites: convertSites(m.cfg.Proxy.GetSites()),
	}, nil
}

type nodeWithSessions struct {
	Node     services.Server   `json:"node"`
	Sessions []session.Session `json:"sessions"`
}

type getSiteNodesResponse struct {
	Nodes []nodeWithSessions `json:"nodes"`
}

/* getSiteNodes returns a list of nodes active in the site

GET /v1/webapi/sites/:site/nodes

Sucessful response:

{"nodes": [
  {
    "node": {
        "addr": "ip:port",
        "hostname": "a.example.com",
        "labels": {"role": "mysql"}, // static key value pairs set by user for every node
        "cmd_labels": {
            "db_status": {
               "command": "mysql -c status", // command periodically executed on server
               "result": "master",  // output of the command
               "period": 1000000000 // microseconds between calls
             }
        }
     },
     "sessions": [{
         "id": "unique session id",
         "parties": [{ // parties is a list of currently active participants
            "id": "party id",
            "user": "alice", // teleport user
            "server_addr": "127.0.0.1:3000",
            "last_active": "time" // RFC3339 timestamp when user was last acive
         }]
     }]
   }
  ]
}
*/
func (m *Handler) getSiteNodes(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	log.Debugf("[web] GET /nodes")
	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers, err := clt.GetNodes()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessions, err := clt.GetSessions()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nodeMap := make(map[string]*nodeWithSessions, len(servers))
	for i := range servers {
		nodeMap[servers[i].ID] = &nodeWithSessions{Node: servers[i]}
	}
	for i := range sessions {
		sess := sessions[i]
		for _, p := range sess.Parties {
			if _, ok := nodeMap[p.ServerID]; ok {
				nodeMap[p.ServerID].Sessions = append(nodeMap[p.ServerID].Sessions, sess)
			}
		}
	}
	nodes := make([]nodeWithSessions, 0, len(nodeMap))
	for key := range nodeMap {
		nodes = append(nodes, *nodeMap[key])
	}
	return getSiteNodesResponse{
		Nodes: nodes,
	}, nil
}

// siteNodeConnect connect to the site node
//
// GET /v1/webapi/sites/:site/connect?access_token=bearer_token&params=<urlencoded json-structure>
//
// Due to the nature of websocket we can't POST parameters as is, so we have
// to add query parameters. The params query parameter is a url encodeed JSON strucrture:
//
// {"server_id": "uuid", "login": "admin", "term": {"h": 120, "w": 100}, "sid": "123"}
//
// Session id can be empty
//
// Sucessful response is a websocket stream that allows read write to the server
//
func (m *Handler) siteNodeConnect(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	q := r.URL.Query()
	params := q.Get("params")
	if params == "" {
		return nil, trace.BadParameter("missing params")
	}
	var req *connectReq
	if err := json.Unmarshal([]byte(params), &req); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("web client connected to node %#v", req)
	connect, err := newConnectHandler(*req, ctx, site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this is to make sure we close web socket connections once
	// sessionContext that owns them expires
	ctx.AddClosers(connect)
	defer connect.Close()
	connect.Handler().ServeHTTP(w, r)
	return nil, nil
}

// sessionStreamEvent is sent over the session stream socket, it contains
// last events that occured (only new events are sent)
type sessionStreamEvent struct {
	Events  []events.EventFields `json:"events"`
	Session *session.Session     `json:"session"`
	Servers []services.Server    `json:"servers"`
}

// siteSessionStream returns a stream of events related to the session
//
// GET /v1/webapi/sites/:site/sessions/:sid/events/stream?access_token=bearer_token
//
// Sucessful response is a websocket stream that allows read write to the server and returns
// json events
//
func (m *Handler) siteSessionStream(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connect, err := newSessionStreamHandler(
		*sessionID, ctx, site, m.sessionStreamPollPeriod)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// this is to make sure we close web socket connections once
	// sessionContext that owns them expires
	ctx.AddClosers(connect)
	defer connect.Close()
	connect.Handler().ServeHTTP(w, r)
	return nil, nil
}

type siteSessionGenerateReq struct {
	Session session.Session `json:"session"`
}

type siteSessionGenerateResponse struct {
	Session session.Session `json:"session"`
}

// siteSessionCreate generates a new site session that can be used by UI
//
// POST /v1/webapi/sites/:site/sessions
//
// Request body:
//
// {"session": {"terminal_params": {"w": 100, "h": 100}, "login": "centos"}}
//
// Response body:
//
// {"session": {"id": "session-id", "terminal_params": {"w": 100, "h": 100}, "login": "centos"}}
//
func (m *Handler) siteSessionGenerate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	var req *siteSessionGenerateReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	req.Session.ID = session.NewID()
	req.Session.Created = time.Now().UTC()
	req.Session.LastActive = time.Now().UTC()
	log.Infof("Generated session: %#v", req.Session)
	return siteSessionGenerateResponse{Session: req.Session}, nil
}

type siteSessionUpdateReq struct {
	TerminalParams session.TerminalParams `json:"terminal_params"`
}

// siteSessionUpdate udpdates the site session
//
// PUT /v1/webapi/sites/:site/sessions/:sid
//
// Request body:
//
// {"terminal_params": {"w": 100, "h": 100}}
//
// Response body:
//
// {"message": "ok"}
//
func (m *Handler) siteSessionUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *siteSessionUpdateReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err = ctx.UpdateSessionTerminal(*sessionID, req.TerminalParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ok(), nil
}

type siteSessionsGetResponse struct {
	Sessions []session.Session `json:"sessions"`
}

// siteSessionGet gets the list of site sessions filtered by creation time
// and either they're active or not
//
// GET /v1/webapi/sites/:site/sessions
//
// Response body:
//
// {"sessions": [{"id": "sid", "terminal_params": {"w": 100, "h": 100}, "parties": [], "login": "bob"}, ...] }
func (m *Handler) siteSessionsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessions, err := clt.GetSessions()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return siteSessionsGetResponse{Sessions: sessions}, nil
}

// siteSessionGet gets the list of site session by id
//
// GET /v1/webapi/sites/:site/sessions/:sid
//
// Response body:
//
// {"session": {"id": "sid", "terminal_params": {"w": 100, "h": 100}, "parties": [], "login": "bob"}}
//
func (m *Handler) siteSessionGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("web.getSetssion(%v)", sessionID)
	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, err := clt.GetSession(*sessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if sess == nil {
		return nil, trace.NotFound("Session %v cannot be found", sessionID)
	}
	return *sess, nil
}

const maxStreamBytes = 5 * 1024 * 1024

// siteEventsGet allows to search for events on site
//
// GET /v1/webapi/sites/:site/events
//
// Query parameters:
//   "from"  : date range from, encoded as RFC3339
//   "to"    : date range to, encoded as RFC3339
//   ...     : the rest of the query string is passed to the search back-end as-is,
//             the default backend performs exact search: ?key=value means "event
//             with a field 'key' with value 'value'
//
func (m *Handler) siteEventsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	query := r.URL.Query()
	log.Infof("web.getEvents(%v)", r.URL.RawQuery)

	clt, err := site.GetClient()
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	to := time.Now().In(time.UTC)
	from := to.AddDate(0, -1, 0) // one month ago

	// parse 'to' and 'from' params:
	fromStr := query.Get("from")
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return nil, trace.BadParameter("from")
		}
	}
	toStr := query.Get("to")
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return nil, trace.BadParameter("to")
		}
	}
	// remove to & from fields, and pass the rest of it directly to the back-end:
	query.Del("to")
	query.Del("from")

	el, err := clt.SearchEvents(from, to, query.Encode())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eventsListGetResponse{Events: el}, nil
}

type siteSessionStreamGetResponse struct {
	Bytes []byte `json:"bytes"`
}

// siteSessionStreamGet returns a byte array from a session's stream
//
// GET /v1/webapi/sites/:site/sessions/:sid/stream?query
//
// Query parameters:
//   "offset"   : bytes from the beginning
//   "bytes"    : number of bytes to read (it won't return more than 512Kb)
//
// Unlike other request handlers, this one does not return JSON.
// It returns the binary stream unencoded, directly in the respose body,
// with Content-Type of application/octet-stream, gzipped with up to 95%
// compression ratio.
func (m *Handler) siteSessionStreamGet(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var site reversetunnel.RemoteSite
	onError := func(err error) {
		w.Header().Set("Content-Type", "text/json")
		trace.WriteError(w, err)
	}
	// authenticate first:
	_, err := m.AuthenticateRequest(w, r, true)
	if err != nil {
		log.Info(err)
		// clear session just in case if the authentication request is not valid
		ClearSession(w)
		onError(err)
		return
	}
	// get the site interface:
	siteName := p.ByName("site")
	if siteName == currentSiteShortcut {
		sites := m.cfg.Proxy.GetSites()
		if len(sites) < 1 {
			onError(trace.NotFound("no active sites"))
			return
		}
		siteName = sites[0].GetName()
	}
	site, err = m.cfg.Proxy.GetSite(siteName)
	if err != nil {
		onError(err)
		return
	}
	// get the session:
	sid, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	clt, err := site.GetClient()
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	// look at 'offset' parameter
	query := r.URL.Query()
	offset, _ := strconv.Atoi(query.Get("offset"))
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	max, err := strconv.Atoi(query.Get("bytes"))
	if err != nil || max <= 0 {
		max = maxStreamBytes
	}
	if max > maxStreamBytes {
		max = maxStreamBytes
	}
	// call the site API to get the chunk:
	bytes, err := clt.GetSessionChunk(*sid, offset, max)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
	// see if we can gzip it:
	var writer io.Writer = w
	for _, acceptedEnc := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(acceptedEnc) == "gzip" {
			gzipper := gzip.NewWriter(w)
			writer = gzipper
			defer gzipper.Close()
			w.Header().Set("Content-Encoding", "gzip")
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = writer.Write(bytes)
	if err != nil {
		onError(trace.Wrap(err))
		return
	}
}

type eventsListGetResponse struct {
	Events []events.EventFields `json:"events"`
}

// siteSessionEventsGet gets the site session by id
//
// GET /v1/webapi/sites/:site/sessions/:sid/events?after=N
//
// Query:
//    "after" : cursor value of an event to return "newer than" events
//              good for repeated polling
//
// Response body (each event is an arbitrary JSON structure)
//
// {"events": [{...}, {...}, ...}
//
func (m *Handler) siteSessionEventsGet(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	sessionID, err := session.ParseID(p.ByName("sid"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := site.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	afterN, err := strconv.Atoi(r.URL.Query().Get("after"))
	if err != nil {
		afterN = 0
	}
	e, err := clt.GetSessionEvents(*sessionID, afterN)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return eventsListGetResponse{Events: e}, nil
}

// createSSHCertReq are passed by web client
// to authenticate against teleport server and receive
// a temporary cert signed by auth server authority
type createSSHCertReq struct {
	// User is a teleport username
	User string `json:"user"`
	// Password is user's pass
	Password string `json:"password"`
	// HOTPToken is second factor token
	HOTPToken string `json:"hotp_token"`
	// PubKey is a public key user wishes to sign
	PubKey []byte `json:"pub_key"`
	// TTL is a desired TTL for the cert (max is still capped by server,
	// however user can shorten the time)
	TTL time.Duration `json:"ttl"`
}

// SSHLoginResponse is a response returned by web proxy
type SSHLoginResponse struct {
	// User contains a logged in user informationn
	Username string `json:"username"`
	// Cert is a signed certificate
	Cert []byte `json:"cert"`
	// HostSigners is a list of signing host public keys
	// trusted by proxy
	HostSigners []services.CertAuthority `json:"host_signers"`
}

// createSSHCert is a web call that generates new SSH certificate based
// on user's name, password, 2nd factor token and public key user wishes to sign
//
// POST /v1/webapi/ssh/certs
//
// { "user": "bob", "password": "pass", "hotp_token": "tok", "pub_key": "key to sign", "ttl": 1000000000 }
//
// Success response
//
// { "cert": "base64 encoded signed cert", "host_signers": [{"domain_name": "example.com", "checking_keys": ["base64 encoded public signing key"]}] }
//
func (h *Handler) createSSHCert(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSSHCertReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := h.auth.GetCertificate(*req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert, nil
}

func (h *Handler) String() string {
	return fmt.Sprintf("multi site")
}

// currentSiteShortcut is a special shortcut that will return the first
// available site, is helpful when UI works in single site mode to reduce
// the amount of requests
const currentSiteShortcut = "-current-"

// contextHandler is a handler called with the auth context, what means it is authenticated and ready to work
type contextHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (interface{}, error)

// siteHandler is a authenticated handler that is called for some existing remote site
type siteHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error)

// withSiteAuth ensures that request is authenticated and is issued for existing site
func (h *Handler) withSiteAuth(fn siteHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		ctx, err := h.AuthenticateRequest(w, r, true)
		if err != nil {
			log.Info(err)
			// clear session just in case if the authentication request is not valid
			ClearSession(w)
			return nil, trace.Wrap(err)
		}
		siteName := p.ByName("site")
		if siteName == currentSiteShortcut {
			sites := h.cfg.Proxy.GetSites()
			if len(sites) < 1 {
				return nil, trace.NotFound("no active sites")
			}
			siteName = sites[0].GetName()
		}
		site, err := h.cfg.Proxy.GetSite(siteName)
		if err != nil {
			log.Warn(err)
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, ctx, site)
	})
}

// withAuth ensures that request is authenticated
func (h *Handler) withAuth(fn contextHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		ctx, err := h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return fn(w, r, p, ctx)
	})
}

// authenticateRequest authenticates request using combination of a session cookie
// and bearer token
func (h *Handler) AuthenticateRequest(w http.ResponseWriter, r *http.Request, checkBearerToken bool) (*SessionContext, error) {
	const missingCookieMsg = "missing session cookie"
	logger := log.WithFields(log.Fields{
		"request": fmt.Sprintf("%v %v", r.Method, r.URL.Path),
	})
	cookie, err := r.Cookie("session")
	if err != nil || (cookie != nil && cookie.Value == "") {
		logger.Infof(missingCookieMsg)
		if err != nil {
			logger.Warn(err)
		}
		return nil, trace.AccessDenied(missingCookieMsg)
	}
	d, err := DecodeCookie(cookie.Value)
	if err != nil {
		logger.Warningf("failed to decode cookie: %v", err)
		return nil, trace.AccessDenied("failed to decode cookie")
	}
	ctx, err := h.auth.ValidateSession(d.User, d.SID)
	if err != nil {
		logger.Warningf("invalid session: %v", err)
		ClearSession(w)
		return nil, trace.AccessDenied("need auth")
	}
	if checkBearerToken {
		creds, err := roundtrip.ParseAuthHeaders(r)
		if err != nil {
			logger.Warningf("no auth headers %v", err)
			return nil, trace.AccessDenied("need auth")
		}
		if creds.Password != ctx.GetWebSession().WS.BearerToken {
			logger.Warningf("bad bearer token")
			return nil, trace.AccessDenied("bad bearer token")
		}
	}
	return ctx, nil
}

func message(msg string) interface{} {
	return map[string]interface{}{"message": msg}
}

func ok() interface{} {
	return message("ok")
}

type Server struct {
	http.Server
}

// CreateSignupLink generates and returns a URL which is given to a new
// user to complete registration with Teleport via Web UI
func CreateSignupLink(hostPort string, token string) string {
	return "https://" + hostPort + "/web/newuser/" + token
}
