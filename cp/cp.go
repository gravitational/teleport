package cp

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/events"
	"github.com/gravitational/teleport/sshutils/scp"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/codahale/lunk"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/form"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/julienschmidt/httprouter"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
)

// CPHandler implements methods for control panel
type CPHandler struct {
	httprouter.Router
	cfg HandlerConfig
}

type HandlerConfig struct {
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

func NewCPHandler(cfg HandlerConfig) *CPHandler {
	if len(cfg.NavSections) == 0 {
		// to avoid panics during iterations
		cfg.NavSections = []NavSection{}
	}
	h := &CPHandler{
		cfg: cfg,
	}

	h.initTemplates(cfg.AssetsDir)
	h.GET("/login", h.login)
	h.GET("/logout", h.logout)
	h.POST("/auth", h.authForm)

	// WEB views
	h.GET("/", h.needsAuth(h.keysIndex))
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

	// Key Management
	h.GET("/api/keys", h.needsAuth(h.getKeys))
	h.POST("/api/keys", h.needsAuth(h.postKey))
	h.DELETE("/api/keys/:key", h.needsAuth(h.deleteKey))

	// Event log
	h.GET("/api/events", h.needsAuth(h.getEvents))
	h.POST("/api/sessions/:id/messages", h.needsAuth(h.sendMessage))

	// Web tunnels
	h.GET("/api/tunnels/web", h.needsAuth(h.getWebTuns))
	h.POST("/api/tunnels/web", h.needsAuth(h.upsertWebTun))
	h.GET("/api/tunnels/web/:prefix", h.needsAuth(h.getWebTun))
	h.DELETE("/api/tunnels/web/:prefix", h.needsAuth(h.deleteWebTun))

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

func (s *CPHandler) needsAuth(fn RequestHandler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		Cookie, err := r.Cookie("session")
		if err != nil {
			log.Infof("error getting Cookie: %v", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		d, err := DecodeCookie(Cookie.Value)
		if err != nil {
			log.Warningf("failed to decode Cookie '%v', err: %v", Cookie.Value, err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx, err := s.cfg.Auth.ValidateSession(d.User, d.SID)
		if err != nil {
			log.Warningf("failed to validate session: %v", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		fn(w, r, p, ctx)
	}
	return nil
}

func (s *CPHandler) ls(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	root := r.URL.Query().Get("node")

	addr := p[0].Value

	up, err := c.ConnectUpstream(addr)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

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

func (s *CPHandler) downloadFiles(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {

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

		up, err := c.ConnectUpstream(addr)
		if err != nil {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}

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
		if err := up.Close(); err != nil && err != io.EOF {
			log.Errorf("file err: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
	}

	ck := &http.Cookie{
		Domain: fmt.Sprintf(".%v", s.cfg.Auth.GetHost()),
		Name:   "fileDownload",
		Value:  "true",
		Path:   "/",
	}
	http.SetCookie(w, ck)
	w.Header().Set("Content-Disposition", "attachment; filename=download.tar")
	utils.WriteArchive(dir, w)
}

func (s *CPHandler) uploadFile(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	file, fh, err := r.FormFile("file")
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	path, addr := r.Form.Get("path"), r.Form.Get("addr")

	up, err := c.ConnectUpstream(addr)
	if err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}

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
	if _, err := io.Copy(f, file); err != nil {
		log.Errorf("file err: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := f.Close(); err != nil {
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
	if err := up.Close(); err != nil {
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

func (s *CPHandler) getSessions(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	ses, err := c.GetClient().GetSessions()
	if err != nil {
		log.Errorf("failed to retrieve sessions: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, ses)
}

func (s *CPHandler) getChunks(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
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
	rdr, err := c.GetClient().GetChunkReader(recordID)
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

func (s *CPHandler) sendMessage(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
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

	c.GetClient().Log(lunk.NewRootEventID(), &events.Message{
		SessionID: sid,
		User:      c.GetUser(),
		Message:   message,
	})

	roundtrip.ReplyJSON(w, http.StatusOK, "ok")
}

func (s *CPHandler) getSession(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	ses, err := c.GetClient().GetSession(p[0].Value)
	if err != nil {
		if !backend.IsNotFound(err) {
			log.Errorf("failed to retrieve session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if err = c.GetClient().UpsertSession(p[0].Value, 60*time.Second); err != nil {
			log.Errorf("failed to upsert session: %v", err)
			replyErr(w, http.StatusInternalServerError, err)
			return
		}
		if ses, err = c.GetClient().GetSession(p[0].Value); err != nil {
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
	events, err := c.GetClient().GetEvents(f)
	if err != nil {
		log.Errorf("failed to retrieve servers: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	srvs, err := c.GetClient().GetServers()
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

func (s *CPHandler) getServers(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	servers, err := c.GetClient().GetServers()
	if err != nil {
		log.Errorf("failed to retrieve servers: %v")
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, servers)
}

func (s *CPHandler) connect(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	log.Infof("connect request authorized to: %v", p[0].Value)
	ws := wsHandler{
		ctx:  c,
		addr: p[0].Value,
		sid:  p[1].Value,
	}
	defer ws.Close()
	ws.Handler().ServeHTTP(w, r)
}

func (s *CPHandler) getWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	tun, err := c.GetClient().GetWebTun(p[0].Value)
	if err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, tun)
}

func (s *CPHandler) deleteWebTun(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	if err := c.GetClient().DeleteWebTun(p[0].Value); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, "deleted")
}

func (s *CPHandler) getWebTuns(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	tuns, err := c.GetClient().GetWebTuns()
	if err != nil {
		log.Errorf("failed to retrieve tunnels: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, tuns)
}

func (s *CPHandler) upsertWebTun(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	var prefix, target, proxy string

	err := form.Parse(r,
		form.String("prefix", &prefix, form.Required()),
		form.String("target", &target, form.Required()),
		form.String("proxy", &proxy, form.Required()))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	wt, err := backend.NewWebTun(prefix, proxy, target)
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	if err := c.GetClient().UpsertWebTun(*wt, 0); err != nil {
		log.Errorf("failed to upsert keys: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, wt)
}

func (s *CPHandler) getEvents(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	f, err := events.FilterFromURL(r.URL.Query())
	if err != nil {
		log.Errorf("failed to retrieve events: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	events, err := c.GetClient().GetEvents(*f)
	if err != nil {
		log.Errorf("failed to retrieve events: %v", err)
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	log.Infof("got events: %v", events)
	roundtrip.ReplyJSON(w, http.StatusOK, events)
}

func (s *CPHandler) keysIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "keys", nil)
}

func (s *CPHandler) eventsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "events", nil)
}

func (s *CPHandler) webTunsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "webtuns", nil)
}

func (s *CPHandler) serversIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "servers", nil)
}

func (s *CPHandler) newSession(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	var server string
	err := form.Parse(r, form.String("server", &server))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	sid := uuid.New()
	if err := c.GetClient().UpsertSession(sid, 30*time.Second); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	u := url.URL{
		Path: s.Path("sessions", sid),
	}
	u.Query().Set("server", server)
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *CPHandler) sessionsIndex(w http.ResponseWriter, r *http.Request, _ httprouter.Params, _ Context) {
	s.executeTemplate(w, "sessions", nil)
}

func (s *CPHandler) sessionIndex(w http.ResponseWriter, r *http.Request, p httprouter.Params, _ Context) {
	s.executeTemplate(w, "session", map[string]interface{}{
		"SessionID":  p[0].Value,
		"ServerAddr": r.URL.Query().Get("server"),
	})
}

func (s *CPHandler) getKeys(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	keys, err := c.GetClient().GetUserKeys(c.GetUser())
	log.Infof("Keys: %v", keys)
	if err != nil {
		log.Errorf("failed to retrieve keys: %v")
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, keys)
}

func (s *CPHandler) postKey(w http.ResponseWriter, r *http.Request, _ httprouter.Params, c Context) {
	var key, id string

	err := form.Parse(r,
		form.String("value", &key, form.Required()),
		form.String("id", &id, form.Required()))
	if err != nil {
		log.Errorf("failed to parse form: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message(err.Error()))
		return
	}
	cert, err := c.GetClient().UpsertUserKey(c.GetUser(), backend.AuthorizedKey{ID: id, Value: []byte(key)}, 0)
	if err != nil {
		log.Errorf("failed to upsert keys: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message("invalid key format"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, backend.AuthorizedKey{ID: key, Value: cert})
}

func (s *CPHandler) deleteKey(w http.ResponseWriter, r *http.Request, p httprouter.Params, c Context) {
	key := p[0].Value

	err := c.GetClient().DeleteUserKey(c.GetUser(), key)
	if err != nil {
		log.Errorf("failed to upsert keys: %v", err)
		roundtrip.ReplyJSON(w, http.StatusBadRequest, message("invalid key format"))
		return
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("key deleted"))
}

func (s *CPHandler) login(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s.executeTemplate(w, "login", nil)
}

func (s *CPHandler) logout(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s.cfg.Auth.ClearSession(w)
	http.Redirect(w, r, s.Path("login"), http.StatusFound)
}

func (s *CPHandler) authForm(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	var user, pass string

	err := form.Parse(r,
		form.String("username", &user, form.Required()),
		form.String("password", &pass, form.Required()))

	if err != nil {
		replyErr(w, http.StatusBadRequest, err)
		return
	}
	sid, err := s.cfg.Auth.Auth(user, pass)
	if err != nil {
		log.Warningf("auth error: %v", err)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if err := s.cfg.Auth.SetSession(w, user, sid); err != nil {
		replyErr(w, http.StatusInternalServerError, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *CPHandler) executeTemplate(w http.ResponseWriter, name string, data map[string]interface{}) {
	if data == nil {
		data = map[string]interface{}{}
	}
	data["Cfg"] = s.cfg
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

type jsNode struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Children bool   `json:"children"`
	Icon     string `json:"icon"`
}
