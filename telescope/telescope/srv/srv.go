package srv

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/lens/Godeps/_workspace/src/github.com/gravitational/session"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/ttlmap"
	"github.com/gravitational/teleport/cp"
	"github.com/gravitational/teleport/tun"
	"github.com/gravitational/teleport/utils"
)

type Handler struct {
	httprouter.Router
	cfg   Config
	auth  cp.AuthHandler
	sites *ttlmap.TtlMap
}

type Config struct {
	Tun         tun.Server
	AssetsDir   string
	CPAssetsDir string
	AuthAddr    utils.NetAddr
	FQDN        string
}

func NewHandler(cfg Config) (*Handler, error) {
	initTemplates(cfg.AssetsDir)

	lauth, err := cp.NewLocalAuth(cfg.FQDN, []utils.NetAddr{cfg.AuthAddr})
	if err != nil {
		return nil, err
	}

	sites, err := ttlmap.NewMap(1024)
	if err != nil {
		return nil, err
	}

	h := &Handler{
		cfg:   cfg,
		auth:  lauth,
		sites: sites,
	}

	// WEB views
	h.GET("/web/login", h.login)
	h.GET("/web/logout", h.logout)
	h.POST("/web/auth", h.authForm)

	h.GET("/", h.needsAuth(h.sitesIndex))
	h.GET("/web/sites", h.needsAuth(h.sitesIndex))

	// Re-use code from the teleport control panel
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

func (h *Handler) String() string {
	return fmt.Sprintf("wshandler")
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	executeTemplate(w, "login", nil)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	if err := session.ClearSession(w, h.cfg.FQDN); err != nil {
		log.Errorf("failed to clear session: %v", err)
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("failed to logout"))
		return
	}
	http.Redirect(w, r, "/web/login", http.StatusFound)
}

func (h *Handler) authForm(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var user, pass string

	err := form.Parse(r,
		form.String("username", &user, form.Required()),
		form.String("password", &pass, form.Required()))

	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	sid, err := h.auth.Auth(user, pass)
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

func (s *Handler) siteEvents(w http.ResponseWriter, r *http.Request, p httprouter.Params, c cp.Context) error {
	executeTemplate(w, "site-events", map[string]interface{}{"SiteName": p[0].Value})
	return nil
}

func (s *Handler) siteServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, c cp.Context) error {
	executeTemplate(w, "site-servers", map[string]interface{}{"SiteName": p[0].Value})
	return nil
}

func (s *Handler) sitesIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c cp.Context) error {
	executeTemplate(w, "sites", nil)
	return nil
}

func (s *Handler) siteHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, c cp.Context) error {
	siteName := p[0].Value
	prefix := fmt.Sprintf("/tun/%v", siteName)
	i, ok := s.sites.Get(siteName)
	if !ok {
		tauth, err := NewTunAuth(s.auth, s.cfg.Tun, siteName)
		if err != nil {
			return err
		}
		i = cp.NewCPHandler(cp.HandlerConfig{
			Auth:      tauth,
			AssetsDir: s.cfg.CPAssetsDir,
			URLPrefix: prefix,
			NavSections: []cp.NavSection{
				cp.NavSection{
					Title: "Back to Lens",
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

func (h *Handler) handleGetSites(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c cp.Context) error {
	roundtrip.ReplyJSON(w, http.StatusOK, sitesResponse(h.cfg.Tun.GetSites()))
	return nil
}

func (h *Handler) needsAuth(fn authHandle) httprouter.Handle {
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

type Server struct {
	http.Server
}

func New(addr utils.NetAddr, cfg Config) (*Server, error) {
	h, err := NewHandler(cfg)
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

func sitesResponse(rs []tun.RemoteSite) []site {
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

func executeTemplate(w http.ResponseWriter, name string, data interface{}) {
	tpl, ok := templates[name]
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

func replyErr(w http.ResponseWriter, code int, err error) {
	roundtrip.ReplyJSON(w, code, message(err.Error()))
}

func message(msg string) map[string]interface{} {
	return map[string]interface{}{"message": msg}
}

type authHandle func(http.ResponseWriter, *http.Request, httprouter.Params, cp.Context) error
