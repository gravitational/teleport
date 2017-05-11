// Package samlidp a rudimentary SAML identity provider suitable for
// testing or as a starting point for a more complex service.
package samlidp

import (
	"net/http"
	"sync"

	"github.com/crewjam/saml"
	"github.com/zenazn/goji/web"
)

// Options represent the parameters to New() for creating a new IDP server
type Options struct {
	URL         string
	Key         string
	Certificate string
	Store       Store
}

// Server represents an IDP server. The server provides the following URLs:
//
//     /metadata     - the SAML metadata
//     /sso          - the SAML endpoint to initiate an authentication flow
//     /login        - prompt for a username and password if no session established
//     /login/:shortcut - kick off an IDP-initiated authentication flow
//     /services     - RESTful interface to Service objects
//     /users        - RESTful interface to User objects
//     /sessions     - RESTful interface to Session objects
//     /shortcuts    - RESTful interface to Shortcut objects
type Server struct {
	http.Handler
	idpConfigMu sync.RWMutex          // protects calls into the IDP
	IDP         saml.IdentityProvider // the underlying IDP
	Store       Store                 // the data store
}

// New returns a new Server
func New(opts Options) (*Server, error) {
	s := &Server{
		IDP: saml.IdentityProvider{
			Key:              opts.Key,
			Certificate:      opts.Certificate,
			MetadataURL:      opts.URL + "/metadata",
			SSOURL:           opts.URL + "/sso",
			ServiceProviders: map[string]*saml.Metadata{},
		},
		Store: opts.Store,
	}
	s.IDP.SessionProvider = s

	if err := s.initializeServices(); err != nil {
		return nil, err
	}
	s.InitializeHTTP()
	return s, nil
}

// InitializeHTTP sets up the HTTP handler for the server. (This function
// is called automatically for you by New, but you may need to call it
// yourself if you don't create the object using New.)
func (s *Server) InitializeHTTP() {
	mux := web.New()
	s.Handler = mux

	mux.Get("/metadata", func(w http.ResponseWriter, r *http.Request) {
		s.idpConfigMu.RLock()
		defer s.idpConfigMu.RUnlock()
		s.IDP.ServeMetadata(w, r)
	})
	mux.Handle("/sso", func(w http.ResponseWriter, r *http.Request) {
		s.idpConfigMu.RLock()
		defer s.idpConfigMu.RUnlock()
		s.IDP.ServeSSO(w, r)
	})

	mux.Handle("/login", s.HandleLogin)
	mux.Handle("/login/:shortcut", s.HandleIDPInitiated)
	mux.Handle("/login/:shortcut/*", s.HandleIDPInitiated)

	mux.Get("/services/", s.HandleListServices)
	mux.Get("/services/:id", s.HandleGetService)
	mux.Put("/services/:id", s.HandlePutService)
	mux.Post("/services/:id", s.HandlePutService)
	mux.Delete("/services/:id", s.HandleDeleteService)

	mux.Get("/users/", s.HandleListUsers)
	mux.Get("/users/:id", s.HandleGetUser)
	mux.Put("/users/:id", s.HandlePutUser)
	mux.Delete("/users/:id", s.HandleDeleteUser)

	mux.Get("/sessions/", s.HandleListSessions)
	mux.Get("/sessions/:id", s.HandleGetSession)
	mux.Delete("/sessions/:id", s.HandleDeleteSession)

	mux.Get("/shortcuts/", s.HandleListShortcuts)
	mux.Get("/shortcuts/:id", s.HandleGetShortcut)
	mux.Put("/shortcuts/:id", s.HandlePutShortcut)
	mux.Delete("/shortcuts/:id", s.HandleDeleteShortcut)
}
