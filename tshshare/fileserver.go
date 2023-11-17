package main

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"sync"
	"text/template"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/tlsca"
)

func newFileServer() (*fileServer, error) {
	fSrv := new(fileServer)

	mux := httprouter.New()
	mux.GET("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if len(r.TLS.VerifiedChains) < 1 {
			log.Error("missing peer cert")
			http.Error(w, "Authentication failed.", http.StatusForbidden)
			return
		}

		id, err := tlsca.FromSubject(r.TLS.PeerCertificates[0].Subject, r.TLS.PeerCertificates[0].NotAfter)
		if err != nil {
			log.WithError(err).Error("invalid tlsca identity")
			http.Error(w, "Authentication failed.", http.StatusForbidden)
			return
		}

		var accessible []string
		fSrv.mu.Lock()

		for shareName, share := range fSrv.shares {
			if share.CanAccess(id.Username, id.Groups) {
				accessible = append(accessible, shareName)
			}
		}
		fSrv.mu.Unlock()

		tmpl := `
		<!DOCTYPE html>
		<html lang="en">
		<body>
			<h2>Welcome to Teleport Connect file sharing!</h2>
			<ul>
				{{range .}}
					<li><a href="{{.}}">{{.}}</a></li>
				{{end}}
			</ul>
		</body>
		</html>
`

		t, err := template.New("index").Parse(tmpl)
		if err != nil {
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}

		err = t.Execute(w, accessible)
		if err != nil {
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
	})
	mux.GET("/:shareName", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		http.Redirect(w, r, r.URL.EscapedPath()+"/", 302)
	})
	mux.GET("/:shareName/*filePath", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		shareName := p.ByName("shareName")
		filePath := p.ByName("filePath")

		if len(r.TLS.VerifiedChains) < 1 {
			log.Error("missing peer cert")
			http.Error(w, "Authentication failed.", http.StatusForbidden)
			return
		}

		id, err := tlsca.FromSubject(r.TLS.PeerCertificates[0].Subject, r.TLS.PeerCertificates[0].NotAfter)
		if err != nil {
			log.WithError(err).Error("invalid tlsca identity")
			http.Error(w, "Authentication failed.", http.StatusForbidden)
			return
		}

		var path string
		fSrv.mu.Lock()
		if share, ok := fSrv.shares[shareName]; ok {
			if share.CanAccess(id.Username, id.Groups) {
				path = share.Path
			} else {
				log.Infof("user %q attempted to access share %q, no access", id.Username, shareName)
			}
		} else {
			log.Infof("user %q attempted to access share %q, does not exist", id.Username, shareName)
		}
		fSrv.mu.Unlock()

		if path == "" {
			http.NotFound(w, r)
			return
		}

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL

		r2.URL.Path = filePath
		r2.URL.RawPath = ""

		log.Infof("all good, serving path %q from share %q to user %q", path, shareName, id.Username)
		// TODO(not espadolini): make the listing prettier
		http.FileServer(customIndexFilesystem{http.Dir(path)}).ServeHTTP(w, r2)
	})

	fSrv.srv = http.Server{
		Handler: mux,
	}

	return fSrv, nil
}

type customIndexFilesystem struct {
	fs http.FileSystem
}

// HTML template for index.html with entries in a table
const indexTemplate = `
<!DOCTYPE html>
<html>
<head>
	<title>File List</title>
	<style>
		table {
			border-collapse: collapse;
			width: 100%;
			margin-top: 10px;
		}
		th, td {
			border: 1px solid #dddddd;
			text-align: left;
			padding: 8px;
		}
		th {
			background-color: #9f85ff;
		}
		.fixed-width {
			width: 200px;
		}
	</style>
</head>
<body>
	<h1>Files in {{.DirName}}</h1>
	<table>
		<tr>
			<th>Name</th>
			<th class="fixed-width">Size (bytes)</th>
			<th class="fixed-width">Modification Time</th>
		</tr>
		{{range .Entries}}
			<tr>
				<td><a href="{{.Name}}">{{if .IsDir}}📁{{else}}📄{{end}} {{.Name | html}}</a></td>
				<td class="fixed-width">{{.Size}}</td>
				<td class="fixed-width">{{.ModTime.Format "2006-01-02 15:04:05"}}</td>
			</tr>
		{{end}}
	</table>
	</body>
</html>
`

func (cifs customIndexFilesystem) Open(filepath string) (outFile http.File, outErr error) {
	// open existing files as usual
	f, errOpen := cifs.fs.Open(filepath)
	if errOpen == nil {
		return f, nil
	}

	// generate custom index.html
	fDir, fName := path.Split(filepath)
	if fName != "index.html" {
		return nil, trace.Wrap(errOpen)
	}

	dir, err := cifs.fs.Open(fDir)
	if err != nil {
		return nil, trace.Wrap(errOpen)
	}

	entries, err := dir.Readdir(-1)
	if err != nil {
		return nil, trace.Wrap(errOpen)
	}

	t, err := template.New("index.html").Parse(indexTemplate)
	if err != nil {
		return nil, trace.Wrap(errOpen)
	}

	slices.SortStableFunc(entries, func(a, b fs.FileInfo) int {
		switch {
		case a.Name() < b.Name():
			return -1
		case a.Name() > b.Name():
			return 1
		}
		return 0
	})

	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}

	slices.SortStableFunc(entries, func(a, b fs.FileInfo) int {
		return b2i(b.IsDir()) - b2i(a.IsDir())
	})

	data := map[string]any{
		"DirName": fDir,
		"Entries": entries,
	}

	var b bytes.Buffer
	err = t.Execute(&b, data)
	if err != nil {
		return nil, trace.Wrap(errOpen)
	}

	return &fixedFile{
		reader: bytes.NewReader(b.Bytes()),
		info: httpFileInfo{
			name: "index.html",
			size: int64(b.Len()),
		},
	}, nil
}

type httpFileInfo struct {
	name string
	size int64
}

func (h httpFileInfo) Name() string {
	return h.name
}

func (h httpFileInfo) Size() int64 {
	return h.size
}

func (h httpFileInfo) Mode() fs.FileMode {
	return 0o644
}

func (h httpFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (h httpFileInfo) IsDir() bool {
	return false
}

func (h httpFileInfo) Sys() any {
	return nil
}

type fixedFile struct {
	reader *bytes.Reader
	info   fs.FileInfo
}

func (f *fixedFile) Close() error {
	return nil
}

func (f *fixedFile) Read(p []byte) (n int, err error) {
	return f.reader.Read(p)
}

func (f *fixedFile) Seek(offset int64, whence int) (int64, error) {
	return f.reader.Seek(offset, whence)
}

func (f *fixedFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, trace.NotImplemented("Readdir not implemented")
}

func (f *fixedFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

var _ http.File = (*fixedFile)(nil)

type fileServer struct {
	srv http.Server

	mu     sync.Mutex
	shares map[string]FileServerShare
}

type FileServerShare struct {
	Path string

	AllowAnyone      bool
	AllowedUsersList []string
	AllowedRolesList []string
}

func (fss FileServerShare) CanAccess(username string, roles []string) bool {
	if fss.AllowAnyone {
		return true
	}

	if slices.Contains(fss.AllowedUsersList, username) {
		return true
	}

	for _, role := range roles {
		if slices.Contains(fss.AllowedRolesList, role) {
			return true
		}
	}

	return false
}

func (f *fileServer) Stop() {
	f.srv.Close()
}
