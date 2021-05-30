package api

import (
	"encoding/base64"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// webHandler is a web handler
type webHandler struct {
	httprouter.Router
}

func (h *webHandler) ping(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	return "ok", nil
}

// newWebHandler returns a web handler
func newWebHandler() (http.Handler, error) {
	h := &webHandler{}

	// ping endpoint is used to check if the server is up. the /webapi/ping
	// endpoint returns the default authentication method and configuration that
	// the server supports. the /webapi/ping/:connector endpoint can be used to
	// query the authentication configuration for a specific connector.
	h.GET("/api/ping", httplib.MakeHandler(h.ping))

	staticFS, err := NewStaticFileSystem()
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

	indexPage, err := template.New("index").Parse(string(indexContent))
	if err != nil {
		return nil, trace.BadParameter("failed parsing index.html template: %v", err)
	}

	routingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// redirect to "/web" when someone hits "/"
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/web", http.StatusFound)
			return
		}

		// serve Web UI, all web properties
		// exist under /web
		//
		// static assets are served from /web/dist path
		if strings.HasPrefix(r.URL.Path, "/web/dist") {
			httplib.SetStaticFileHeaders(w.Header())
			http.StripPrefix("/web/dist", http.FileServer(staticFS)).ServeHTTP(w, r)
		} else if strings.HasPrefix(r.URL.Path, "/web/") || r.URL.Path == "/web" {
			// dynamic pages are served via /web/ root path
			csrfToken, err := csrf.AddCSRFProtection(w, r)
			if err != nil {
				log.Errorf("failed to generate CSRF token %v", err)
			}
			session := struct {
				Session string
				XCSRF   string
			}{
				XCSRF:   csrfToken,
				Session: base64.StdEncoding.EncodeToString([]byte("{}")),
			}

			httplib.SetIndexHTMLHeaders(w.Header())
			indexPage.Execute(w, session)
		} else {
			http.NotFound(w, r)
		}
	})

	h.NotFound = routingHandler
	return h, nil
}

// NewStaticFileSystem returns the initialized implementation of http.FileSystem
// interface which can be used to serve Teleport Proxy Web UI
//
// If 'debugMode' is true, it will load the web assets from the same git repo
// directory where the executable is, otherwise it will load them from the embedded
// zip archive.
//
func NewStaticFileSystem() (http.FileSystem, error) {
	assetsToCheck := []string{"index.html"}

	assetsPath := "./webapps/packages/term/dist"
	for _, af := range assetsToCheck {
		_, err := os.Stat(filepath.Join(assetsPath, af))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	log.Infof("[Web] Using filesystem for serving web assets: %s", assetsPath)
	return http.Dir(assetsPath), nil
}
