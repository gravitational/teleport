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
package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/mailgun/ttlmap"
)

type MultiSiteHandler struct {
	httprouter.Router
	cfg       MultiSiteConfig
	auth      AuthHandler
	sites     *ttlmap.TtlMap
	templates map[string]*template.Template
	sync.Mutex
}

type MultiSiteConfig struct {
	InsecureHTTPMode bool
	Tun              reversetunnel.Server
	AssetsDir        string
	AuthAddr         utils.NetAddr
	DomainName       string
}

const Version = "v1"

func NewMultiSiteHandler(cfg MultiSiteConfig) (http.Handler, error) {
	lauth, err := NewLocalAuth(!cfg.InsecureHTTPMode, []utils.NetAddr{cfg.AuthAddr})
	if err != nil {
		return nil, err
	}

	sites, err := ttlmap.NewMap(1024)
	if err != nil {
		return nil, err
	}

	h := &MultiSiteHandler{
		cfg:   cfg,
		auth:  lauth,
		sites: sites,
	}

	// Web sessions
	h.POST("/webapi/sessions", httplib.MakeHandler(h.createSession))
	h.DELETE("/webapi/sessions/:sid", h.needsAuth(h.deleteSession))

	// Users
	h.GET("/webapi/users/invites/:token", httplib.MakeHandler(h.renderUserInvite))
	h.POST("/webapi/users", httplib.MakeHandler(h.createNewUser))

	// SSH proxy web login
	h.POST("/sshlogin", h.loginSSHProxy)

	// Forward all requests to site handler
	sh := h.needsAuth(h.siteHandler)
	h.GET("/webapi/sites/:site/*path", sh)
	h.PUT("/webapi/sites/:site/*path", sh)
	h.POST("/webapi/sites/:site/*path", sh)
	h.DELETE("/webapi/sites/:site/*path", sh)

	routingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/web/app") {
			http.StripPrefix("/web", http.FileServer(http.Dir(cfg.AssetsDir))).ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web") {
			http.ServeFile(w, r, filepath.Join(cfg.AssetsDir, "/index.html"))
		} else if strings.HasPrefix(r.URL.Path, "/"+Version) {
			http.StripPrefix("/"+Version, h).ServeHTTP(w, r)
		}
	})

	return routingHandler, nil
}

// createSessionReq is a request to create session from username, password and second
// factor token
type createSessionReq struct {
	User              string `json:"user"`
	Pass              string `json:"pass"`
	SecondFactorToken string `json:"second_factor_token"`
}

// createSessionResponse returns OAuth compabible data about
// access token: https://tools.ietf.org/html/rfc6749
type createSessionResponse struct {
	// Type is token type (bearer)
	Type string `json:"type"`
	// Token value
	Token string `json:"token"`
	// User represents the user
	User string `json:"user"`
	// ExpiresIn sets seconds before this token is not valid
	ExpiresIn int `json:"expires_in"`
}

// createSession creates a new web session based on user, pass and 2nd factor token
//
// POST /v1/webapi/sessions
//
// {"user": "alex", "pass": "abc123", "second_factor_token": "token"}
//
// Response
//
// {"type": "bearer", "token": "bearer token", "user": "alex", "expires_in": 20}
//
func (m *MultiSiteHandler) createSession(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createSessionReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := m.auth.Auth(req.User, req.Pass, req.SecondFactorToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := SetSession(w, req.User, sess.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	return &createSessionResponse{
		Type:      roundtrip.AuthBearer,
		Token:     sess.ID,
		User:      req.User,
		ExpiresIn: int(time.Now().Sub(sess.WS.Expires) / time.Second),
	}, nil
}

// deleteSession is called to sign out user
//
// DELETE /v1/web/sessions/:sid
//
// Response:
//
// {"message": "ok"}
//
func (m *MultiSiteHandler) deleteSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, ctx Context) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess := ctx.GetWebSession()
	if err := clt.DeleteWebSession(ctx.GetUser(), sess.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := ClearSession(w); err != nil {
		return nil, trace.Wrap(err)
	}
	return ok(), nil
}

type renderUserInviteResponse struct {
	InviteToken string `json:"invite_token"`
	User        string `json:"user"`
	QR          []byte `json:"qr"`
}

// renderUserInvite is called to show user the new user invitation page
//
// GET /v1/web/users/invites/:token
//
// Response:
//
// {"invite_token": "token", "user": "alex", qr: "base64-encoded-qr-code image"}
//
//
func (m *MultiSiteHandler) renderUserInvite(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
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
// POST /v1/web/users
//
// {"invite_token": "unique invite token", "pass": "user password", "second_factor_token": "valid second factor token"}
//
// Sucessful response: (session cookie is set)
//
// {"type": "bearer", "token": "bearer token", "user": "alex", "expires_in": 20}
func (m *MultiSiteHandler) createNewUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req *createNewUserReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	sess, err := m.auth.CreateNewUser(req.InviteToken, req.Pass, req.SecondFactorToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := SetSession(w, sess.User, sess.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	return &createSessionResponse{
		Type:      roundtrip.AuthBearer,
		Token:     sess.ID,
		User:      sess.User,
		ExpiresIn: int(time.Now().Sub(sess.WS.Expires) / time.Second),
	}, nil
}

func message(msg string) interface{} {
	return map[string]interface{}{"message": msg}
}

func ok() interface{} {
	return message("ok")
}

func (s *MultiSiteHandler) initTemplates(baseDir string) {
	s.Lock()
	defer s.Unlock()
	if len(s.templates) != 0 {
		return
	}
	tpls := []tpl{
		tpl{name: "login", include: []string{"assets/static/tpl/login.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "error", include: []string{"assets/static/tpl/error.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "newuser", include: []string{"assets/static/tpl/newuser.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "sites", include: []string{"assets/static/tpl/sites.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "site-servers", include: []string{"assets/static/tpl/site/servers.tpl", "assets/static/tpl/base.tpl"}},
		tpl{name: "site-events", include: []string{"assets/static/tpl/site/events.tpl", "assets/static/tpl/base.tpl"}},
	}
	s.templates = make(map[string]*template.Template)
	for _, t := range tpls {
		tpl := template.New(t.name)
		for _, i := range t.include {
			template.Must(tpl.ParseFiles(filepath.Join(baseDir, i)))
		}
		s.templates[t.name] = tpl
	}
}

func (h *MultiSiteHandler) String() string {
	return fmt.Sprintf("multi site")
}

func (h *MultiSiteHandler) newUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	token := p[0].Value
	user, QRImg, _, err := h.auth.GetUserInviteInfo(token)
	if err != nil {
		http.Redirect(w, r, ErrorPageLink("Signup link had expired"),
			http.StatusFound)
		return
	}

	base64QRImg := base64.StdEncoding.EncodeToString(QRImg)
	h.executeTemplate(w, "newuser", map[string]interface{}{
		"Token":    token,
		"Username": user,
		"QR":       base64QRImg})
}

func (h *MultiSiteHandler) finishNewUser(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var token, pass, pass2, hotpToken string

	err := form.Parse(r,
		form.String("token", &token, form.Required()),
		form.String("password", &pass, form.Required()),
		form.String("password_confirm", &pass2, form.Required()),
		form.String("hotp_token", &hotpToken, form.Required()),
	)

	if err != nil {
		http.Redirect(w, r, ErrorPageLink("Error: "+err.Error()),
			http.StatusFound)
		return
	}

	if pass != pass2 {
		http.Redirect(w, r, ErrorPageLink("Provided passwords mismatch"),
			http.StatusFound)
		return
	}

	_, err = h.auth.CreateNewUser(token, pass, hotpToken)
	if err != nil {
		if strings.Contains(err.Error(), "Wrong HOTP token") {
			http.Redirect(w, r, ErrorPageLink("Wrong HOTP token"),
				http.StatusFound)
		} else {
			http.Redirect(w, r, ErrorPageLink("Error: "+err.Error()),
				http.StatusFound)
		}
		return
	}

	http.Redirect(w, r, "/web/loginaftercreation", http.StatusFound)
}

func (h *MultiSiteHandler) login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	h.executeTemplate(w, "login", nil)
}

func (h *MultiSiteHandler) loginError(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	h.executeTemplate(w, "login", map[string]interface{}{"ErrorString": "Wrong username or password or hotp token"})
}

func (h *MultiSiteHandler) loginAfterCreation(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	h.executeTemplate(w, "login", map[string]interface{}{"InfoString": "Account was successfully created, you can login"})
}

func (h *MultiSiteHandler) errorPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	errorString := r.URL.Query().Get("message")
	h.executeTemplate(w, "error", map[string]interface{}{"ErrorString": errorString})
}

func ErrorPageLink(message string) string {
	return "/web/error?message=" + url.QueryEscape(message)
}

func (h *MultiSiteHandler) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := ClearSession(w); err != nil {
		log.Errorf("failed to clear session: %v", err)
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("failed to logout"))
		return
	}
	http.Redirect(w, r, "/web/login", http.StatusFound)
}

func (h *MultiSiteHandler) authForm(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var user, pass, hotpToken string

	err := form.Parse(r,
		form.String("username", &user, form.Required()),
		form.String("password", &pass, form.Required()),
		form.String("hotpToken", &hotpToken, form.Required()),
	)

	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	sess, err := h.auth.Auth(user, pass, hotpToken)
	if err != nil {
		log.Warningf("auth error: %v", err)
		http.Redirect(w, r, "/web/loginerror", http.StatusFound)
		return
	}
	if err := SetSession(w, user, sess.ID); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *MultiSiteHandler) loginSSHProxy(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var credJSON string

	err := form.Parse(r,
		form.String("credentials", &credJSON, form.Required()),
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(trace.Wrap(err).Error()))
		return
	}

	var cred SSHLoginCredentials
	if err := json.Unmarshal([]byte(credJSON), &cred); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(trace.Wrap(err).Error()))
		return
	}

	cert, err := h.auth.GetCertificate(cred)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(trace.Wrap(err).Error()))
		return
	}
	out, err := json.Marshal(cert)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(trace.Wrap(err).Error()))
		return
	}
	w.Write(out)
}

func (s *MultiSiteHandler) siteEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) error {
	s.executeTemplate(w, "site-events", map[string]interface{}{"SiteName": p[0].Value})
	return nil
}

func (s *MultiSiteHandler) siteServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) error {
	s.executeTemplate(w, "site-servers", map[string]interface{}{"SiteName": p[0].Value})
	return nil
}

func (s *MultiSiteHandler) sitesIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) (interface{}, error) {
	s.executeTemplate(w, "sites", nil)
	return nil, nil
}

func (s *MultiSiteHandler) siteHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) (interface{}, error) {
	siteName := p[0].Value
	prefix := fmt.Sprintf("/tun/%v", siteName)
	i, ok := s.sites.Get(siteName)
	if !ok {
		tauth, err := NewTunAuth(s.auth, s.cfg.Tun, siteName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		i = NewSiteHandler(SiteHandlerConfig{
			Auth:      tauth,
			AssetsDir: s.cfg.AssetsDir,
			URLPrefix: prefix,
			NavSections: []NavSection{
				NavSection{
					Title: "Back to Portal",
					Icon:  "fa fa-arrow-circle-left",
					URL:   "/",
				},
			},
		})
		if err := s.sites.Set(siteName, i, 90); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	sh := i.(http.Handler)
	r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
	r.RequestURI = r.URL.String()
	sh.ServeHTTP(w, r)
	return nil, nil
}

// contextHandler is a handler called with the auth context, what means it is authenticated and ready to work
type contextHandler func(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx Context) (interface{}, error)

func (h *MultiSiteHandler) needsAuth(fn contextHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
		cookie, err := r.Cookie("session")
		if err != nil {
			return nil, trace.Wrap(teleport.AccessDenied("missing cookie"))
		}
		d, err := DecodeCookie(cookie.Value)
		if err != nil {
			return nil, trace.Wrap(teleport.AccessDenied("failed to decode cookie"))
		}
		ctx, err := h.auth.ValidateSession(d.User, d.SID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		creds, err := roundtrip.ParseAuthHeaders(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if creds.Password != d.SID {
			return nil, trace.Wrap(teleport.AccessDenied("missing auth token"))
		}
		return fn(w, r, p, ctx)
	})
}

func (h *MultiSiteHandler) executeTemplate(w http.ResponseWriter, name string, data interface{}) {
	h.initTemplates(h.cfg.AssetsDir)
	tpl, ok := h.templates[name]
	if !ok {
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("template '%v' not found", name))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tpl.ExecuteTemplate(w, "base", data); err != nil {
		log.Errorf("Execute template: %v", err)
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("internal render error"))
	}
}

type Server struct {
	http.Server
}

func New(addr utils.NetAddr, cfg MultiSiteConfig) (*Server, error) {
	h, err := NewMultiSiteHandler(cfg)
	if err != nil {
		return nil, err
	}
	srv := &Server{}
	srv.Server.Addr = addr.Addr
	srv.Server.Handler = h
	return srv, nil
}

type site struct {
	Name          string    `json:"name"`
	LastConnected time.Time `json:"last_connected"`
	Status        string    `json:"status"`
}

func sitesResponse(rs []reversetunnel.RemoteSite) []site {
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

func CreateSignupLink(hostPort string, token string) string {
	return "http://" + hostPort + "/web/newuser/" + token
}
