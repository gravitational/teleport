package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/ttlmap"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
)

type MultiSiteHandler struct {
	httprouter.Router
	cfg       MultiSiteConfig
	auth      AuthHandler
	sites     *ttlmap.TtlMap
	templates map[string]*template.Template
}

type MultiSiteConfig struct {
	Tun       reversetunnel.Server
	AssetsDir string
	AuthAddr  utils.NetAddr
	FQDN      string
}

func NewMultiSiteHandler(cfg MultiSiteConfig) (*MultiSiteHandler, error) {
	lauth, err := NewLocalAuth(cfg.FQDN, []utils.NetAddr{cfg.AuthAddr})
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

	h.initTemplates(cfg.AssetsDir)

	// WEB views
	h.GET("/web/login", h.login)
	h.GET("/web/logout", h.logout)
	h.POST("/web/auth", h.authForm)

	h.GET("/", h.needsAuth(h.sitesIndex))
	h.GET("/web/sites", h.needsAuth(h.sitesIndex))

	// Forward all requests to site handler
	sh := h.needsAuth(h.siteHandler)
	h.GET("/tun/:site/*path", sh)
	h.PUT("/tun/:site/*path", sh)
	h.POST("/tun/:site/*path", sh)
	h.DELETE("/tun/:site/*path", sh)

	// API views
	h.GET("/api/sites", h.needsAuth(h.handleGetSites))

	// Static assets
	h.Handler("GET", "/static/*filepath",
		http.FileServer(http.Dir(filepath.Join(cfg.AssetsDir, "assets"))))
	return h, nil
}

func (s *MultiSiteHandler) initTemplates(baseDir string) {
	tpls := []tpl{
		tpl{name: "login", include: []string{"assets/static/tpl/login.tpl", "assets/static/tpl/base.tpl"}},
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

func (h *MultiSiteHandler) login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	h.executeTemplate(w, "login", nil)
}

func (h *MultiSiteHandler) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := session.ClearSession(w, h.cfg.FQDN); err != nil {
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
	sid, err := h.auth.Auth(user, pass, hotpToken)
	if err != nil {
		log.Warningf("auth error: %v", err)
		http.Redirect(w, r, "/web/login", http.StatusFound)
		return
	}
	if err := session.SetSession(w, h.cfg.FQDN, user, sid); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *MultiSiteHandler) siteEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) error {
	s.executeTemplate(w, "site-events", map[string]interface{}{"SiteName": p[0].Value})
	return nil
}

func (s *MultiSiteHandler) siteServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) error {
	s.executeTemplate(w, "site-servers", map[string]interface{}{"SiteName": p[0].Value})
	return nil
}

func (s *MultiSiteHandler) sitesIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) error {
	s.executeTemplate(w, "sites", nil)
	return nil
}

func (s *MultiSiteHandler) siteHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) error {
	siteName := p[0].Value
	prefix := fmt.Sprintf("/tun/%v", siteName)
	i, ok := s.sites.Get(siteName)
	if !ok {
		tauth, err := NewTunAuth(s.auth, s.cfg.Tun, siteName)
		if err != nil {
			return err
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
			return err
		}
	}
	sh := i.(http.Handler)
	r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
	r.RequestURI = r.URL.String()
	log.Infof("siteHandler: %v %v", r.Method, r.URL)
	sh.ServeHTTP(w, r)
	return nil
}

func (h *MultiSiteHandler) handleGetSites(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) error {
	roundtrip.ReplyJSON(w, http.StatusOK, sitesResponse(h.cfg.Tun.GetSites()))
	return nil
}

func (h *MultiSiteHandler) needsAuth(fn authHandle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		cookie, err := r.Cookie("session")
		if err != nil {
			log.Infof("getting cookie: %v", err)
			http.Redirect(w, r, "/web/login", http.StatusFound)
			return
		}
		d, err := session.DecodeCookie(cookie.Value)
		if err != nil {
			log.Warningf("failed to decode cookie '%v', err: %v", cookie.Value, err)
			http.Redirect(w, r, "/web/login", http.StatusFound)
			return
		}
		ctx, err := h.auth.ValidateSession(d.User, d.SID)
		if err != nil {
			log.Warningf("failed to validate session: %v", err)
			http.Redirect(w, r, "/web/login", http.StatusFound)
			return
		}
		if err := fn(w, r, p, ctx); err != nil {
			log.Errorf("error in handler: %v", err)
			roundtrip.ReplyJSON(
				w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	return nil
}

func (h *MultiSiteHandler) executeTemplate(w http.ResponseWriter, name string, data interface{}) {
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

type authHandle func(http.ResponseWriter, *http.Request, httprouter.Params, Context) error
