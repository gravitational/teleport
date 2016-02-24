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
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	"github.com/julienschmidt/httprouter"
	"github.com/pborman/uuid"
)

// SiteHandler implements methods for single site
type SiteHandler struct {
	httprouter.Router
	cfg       SiteHandlerConfig
	templates map[string]*template.Template
	sync.Mutex
}

type SiteHandlerConfig struct {
	Auth        AuthHandler
	AssetsDir   string
	NavSections []NavSection
	URLPrefix   string
}

type NavSection struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

func NewSiteHandler(cfg SiteHandlerConfig) *SiteHandler {
	if len(cfg.NavSections) == 0 {
		// to avoid panics during iterations
		cfg.NavSections = []NavSection{}
	}
	h := &SiteHandler{
		cfg: cfg,
	}

	h.GET("/login", h.login)
	h.GET("/logout", h.logout)
	h.POST("/auth", h.authForm)

	// WEB views
	h.GET("/", h.needsAuth(h.serversIndex))
	h.GET("/keys", h.needsAuth(h.keysIndex))
	h.GET("/events", h.needsAuth(h.eventsIndex))
	h.GET("/webtuns", h.needsAuth(h.webTunsIndex))
	h.GET("/servers", h.needsAuth(h.serversIndex))
	h.GET("/sessions", h.needsAuth(h.sessionsIndex))
	h.POST("/sessions", h.needsAuth(h.newSession))
	h.GET("/sessions/:id", h.needsAuth(h.sessionIndex))

	h.POST("/servers/:id/files", h.needsAuth(h.uploadFile))
	h.GET("/servers/:id/ls", h.needsAuth(h.ls))
	h.GET("/servers/:id/download", h.needsAuth(h.downloadFiles))

	// JSON API methods

	// Event log
	h.GET("/api/events", h.needsAuth(h.getEvents))
	h.POST("/api/sessions/:id/messages", h.needsAuth(h.sendMessage))

	// Remote access to SSH server
	h.GET("/api/ssh/connect/:server/sessions/:sid", h.needsAuth(h.connect))

	// Operations with servers
	h.GET("/api/servers", h.needsAuth(h.getServers))

	// Static assets
	h.Handler("GET", "/static/*filepath",
		http.FileServer(http.Dir(filepath.Join(cfg.AssetsDir, "assets"))))

	// Operations with sessions
	h.GET("/api/sessions", h.needsAuth(h.getSessions))
	h.GET("/api/sessions/:id", h.needsAuth(h.getSession))

	// Records playback
	h.GET("/api/records/:id/chunks", h.needsAuth(h.getChunks))

	return h
}

func (s *SiteHandler) needsAuth(fn RequestHandler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		Cookie, err := r.Cookie("session")
		if err != nil {
			log.Infof("error getting Cookie: %v", err)
			http.Redirect(w, r, "/web/login", http.StatusFound)
			return
		}
		d, err := DecodeCookie(Cookie.Value)
		if err != nil {
			log.Warningf("failed to decode Cookie '%v', err: %v", Cookie.Value, err)
			http.Redirect(w, r, "/web/login", http.StatusFound)
			return
		}
		ctx, err := s.cfg.Auth.ValidateSession(d.User, d.SID)
		if err != nil {
			log.Warningf("failed to validate session: %v", err)
			http.Redirect(w, r, "/web/login", http.StatusFound)
			return
		}
		fn(w, r, p, ctx)
	}
}

func (s *SiteHandler) ls(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	root := r.URL.Query().Get("node")

	addr := p[0].Value

	up, err := c.ConnectUpstream(addr, "TODO_PUT_OS_USER_HERE")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer up.Close()

	session := up.GetSession()

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := session.RequestSubsystem(fmt.Sprintf("ls:%v", root)); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	out, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	var nodes []utils.FileNode
	if err := json.Unmarshal(out, &nodes); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	jsnodes := make([]interface{}, len(nodes))
	for i, n := range nodes {
		var icon string
		if n.Dir {
			icon = "fa fa-folder"
		} else {
			icon = "fa fa-file-code-o"
		}
		jsnodes[i] = jsNode{
			ID:       filepath.Join(n.Parent, n.Name),
			Text:     n.Name,
			Children: n.Dir,
			Icon:     icon,
		}

	}
	roundtrip.ReplyJSON(w, http.StatusOK, jsnodes)
}

func (s *SiteHandler) downloadFiles(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {

	addr := p[0].Value

	files := r.URL.Query()["path"]
	if len(files) == 0 {
		replyErr(w, http.StatusInternalServerError, fmt.Errorf("need some files"))
		return
	}

	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Infof("failed to remove temp file")
		}
	}()

	for _, p := range files {
		target := filepath.Join(dir, filepath.Dir(p))
		if err := os.MkdirAll(target, 0755); err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}

		up, err := c.ConnectUpstream(addr, "TODO_PUT_OS_USER_HERE")
		if err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		defer func() {
			if err := up.Close(); err != nil && err != io.EOF {
				log.Errorf("file err: %v", err)
			}
		}()
		rw, err := up.CommandRW(fmt.Sprintf("scp -v -f %v", p))
		uploader, err := scp.New(scp.Command{Sink: true, Target: dir})
		if err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}

		if err := uploader.Serve(rw); err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}

	ck := &http.Cookie{
		Name:  "fileDownload",
		Value: "true",
		Path:  "/",
	}
	http.SetCookie(w, ck)
	w.Header().Set("Content-Disposition", "attachment; filename=download.tar")
	utils.WriteArchive(dir, w)
}

func (s *SiteHandler) uploadFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	file, fh, err := r.FormFile("file")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	path, addr := r.Form.Get("path"), r.Form.Get("addr")

	up, err := c.ConnectUpstream(addr, "TODO_PUT_OS_USER_HERE")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer up.Close()

	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	fpath := filepath.Join(dir, fh.Filename)

	f, err := os.Create(fpath)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer f.Close()
	if _, err := io.Copy(f, file); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Infof("failed to remove temp file")
		}
	}()

	rw, err := up.CommandRW(fmt.Sprintf("scp -v -t %v", path))
	uploader, err := scp.New(scp.Command{Source: true, Target: f.Name()})
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := uploader.Serve(rw); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	log.Infof("%v uploaded", fh.Filename)
	res := map[string]interface{}{
		"result": map[string]interface{}{
			"name": fh.Filename,
		},
	}
	roundtrip.ReplyJSON(w, http.StatusOK, res)
}

func (s *SiteHandler) getSessions(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	ses, err := clt.GetSessions()
	if err != nil {
		log.Errorf("failed to retrieve sessions: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, ses)
}

func (s *SiteHandler) getChunks(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	st, en := r.URL.Query().Get("start"), r.URL.Query().Get("end")
	start, err := strconv.Atoi(st)
	if err != nil {
		log.Errorf("failed to convert: %v", err)
		replyErr(w, http.StatusBadRequest,
			fmt.Errorf("failed to convert"))
		return
	}
	end, err := strconv.Atoi(en)
	if err != nil {
		log.Errorf("failed to convert: %v", err)
		replyErr(w, http.StatusBadRequest,
			fmt.Errorf("failed to convert"))
		return
	}

	recordID := p[0].Value
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	rdr, err := clt.GetChunkReader(recordID)
	if err != nil {
		log.Errorf("failed to retrieve reader: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer rdr.Close()
	chunks, err := rdr.ReadChunks(start, end)
	if err != nil {
		log.Errorf("failed to get chunks: %v", err)
		replyErr(w, http.StatusInternalServerError,
			fmt.Errorf("failed to get chunks"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, chunks)
}

func (s *SiteHandler) sendMessage(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	sid := p[0].Value

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("failed to retrieve sessions: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	var message string
	if err := json.Unmarshal(b, &message); err != nil {
		log.Errorf("failed to unmarshal: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	clt.Log(lunk.NewRootEventID(), &events.Message{
		SessionID: sid,
		User:      c.GetUser(),
		Message:   message,
	})

	roundtrip.ReplyJSON(w, http.StatusOK, "ok")
}

func (s *SiteHandler) getSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	ses, err := clt.GetSession(p[0].Value)
	if err != nil {
		if !teleport.IsNotFound(err) {
			log.Errorf("failed to retrieve session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if err = clt.UpsertSession(p[0].Value, 60*time.Second); err != nil {
			log.Errorf("failed to upsert session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if ses, err = clt.GetSession(p[0].Value); err != nil {
			log.Errorf("failed to upsert session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}
	f := events.Filter{
		SessionID: p[0].Value,
		Order:     events.Desc,
		Limit:     20,
		Start:     time.Now(),
	}
	events, err := clt.GetEvents(f)
	if err != nil {
		log.Errorf("failed to retrieve servers: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	srvs, err := clt.GetServers()
	if err != nil {
		log.Errorf("failed to retrieve servers: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK,
		map[string]interface{}{
			"session": ses,
			"servers": srvs,
			"events":  events,
		})
}

func (s *SiteHandler) getServers(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	servers, err := clt.GetServers()
	if err != nil {
		log.Errorf("failed to retrieve servers: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, servers)
}

func (s *SiteHandler) connect(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	log.Infof("connect request authorized to: %v", p[0].Value)
	ws := wsHandler{
		ctx:  c,
		addr: p[0].Value,
		sid:  p[1].Value,
	}
	defer ws.Close()
	ws.Handler().ServeHTTP(w, r)
}

func (s *SiteHandler) getEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	f, err := events.FilterFromURL(r.URL.Query())
	if err != nil {
		log.Errorf("failed to retrieve events: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	events, err := clt.GetEvents(*f)
	if err != nil {
		log.Errorf("failed to retrieve events: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, events)
}

func (s *SiteHandler) keysIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "keys", nil)
}

func (s *SiteHandler) eventsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "events", nil)
}

func (s *SiteHandler) webTunsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "webtuns", nil)
}

func (s *SiteHandler) serversIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "servers", nil)
}

func (s *SiteHandler) newSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	var server string
	err := form.Parse(r, form.String("server", &server))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	sid := uuid.New()
	clt, err := c.GetClient()
	if err != nil {
		log.Errorf("failed to get client: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := clt.UpsertSession(sid, 30*time.Second); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	u := url.URL{
		Path: s.Path("sessions", sid),
	}
	u.Query().Set("server", server)
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *SiteHandler) sessionsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "sessions", nil)
}

func (s *SiteHandler) sessionIndex(w http.ResponseWriter, r *http.Request, p httprouter.Params, _ Context) {
	s.executeTemplate(w, "session", map[string]interface{}{
		"SessionID":  p[0].Value,
		"ServerAddr": r.URL.Query().Get("server"),
	})
}

func (s *SiteHandler) login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s.executeTemplate(w, "login", nil)
}

func (s *SiteHandler) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s.cfg.Auth.ClearSession(w)
	http.Redirect(w, r, s.Path("login"), http.StatusFound)
}

func (s *SiteHandler) authForm(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
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
	sess, err := s.cfg.Auth.Auth(user, pass, hotpToken)
	if err != nil {
		log.Warningf("auth error: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := s.cfg.Auth.SetSession(w, user, sess.ID); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *SiteHandler) executeTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	s.initTemplates(s.cfg.AssetsDir)
	if data == nil {
		data = map[string]interface{}{}
	}
	data["Cfg"] = s.cfg
	tpl, ok := s.templates[name]
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

func (s *SiteHandler) Path(params ...string) string {
	out := append([]string{"/", s.cfg.URLPrefix}, params...)
	return filepath.Join(out...)
}

func (s *SiteHandler) initTemplates(baseDir string) {
	s.Lock()
	defer s.Unlock()

	if len(s.templates) != 0 {
		return
	}

	tpls := []tpl{
		tpl{name: "login", include: []string{"assets/static/tpl/login.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "keys", include: []string{"assets/static/tpl/keys.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "events", include: []string{"assets/static/tpl/events.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "webtuns", include: []string{"assets/static/tpl/webtuns.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "servers", include: []string{"assets/static/tpl/servers.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "sessions", include: []string{"assets/static/tpl/sessions.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "session", include: []string{"assets/static/tpl/session.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "sites", include: []string{"assets/static/tpl/sites.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "site-servers", include: []string{"assets/static/tpl/site/servers.tpl", "assets/static/tpl/site-base.tpl"}},
		tpl{name: "site-events", include: []string{"assets/static/tpl/site/events.tpl", "assets/static/tpl/site-base.tpl"}},
	}
	s.templates = make(map[string]*template.Template)
	for _, t := range tpls {
		tpl := template.New(t.name)
		tpl.Funcs(map[string]interface{}{
			"Path": s.Path,
		})
		for _, i := range t.include {
			template.Must(tpl.ParseFiles(filepath.Join(baseDir, i)))
		}
		s.templates[t.name] = tpl
	}
}

type tpl struct {
	name    string
	include []string
}

func replyErr(w http.ResponseWriter, code int, err error) {
	roundtrip.ReplyJSON(w, code, message(err.Error()))
}

type jsNode struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Children bool   `json:"children"`
	Icon     string `json:"icon"`
}
