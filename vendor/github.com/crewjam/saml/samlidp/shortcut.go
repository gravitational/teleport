package samlidp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/zenazn/goji/web"
)

// Shortcut represents an IDP-initiated SAML flow. When a user
// navigates to /login/:shortcut it initiates the login flow
// to the specified service provider with the specified
// RelayState.
type Shortcut struct {
	// The name of the shortcut.
	Name string `json:"name"`

	// The entity ID of the service provider to use for this shortcut, i.e.
	// https://someapp.example.com/saml/metadata.
	ServiceProviderID string `json:"service_provider"`

	// If specified then the relay state is the fixed string provided
	RelayState *string `json:"relay_state,omitempty"`

	// If true then the URL suffix is used as the relayState. So for example, a user
	// requesting https://idp.example.com/login/myservice/foo will get redirected
	// to the myservice endpoint with a RelayState of "foo".
	URISuffixAsRelayState bool `json:"url_suffix_as_relay_state,omitempty"`
}

// HandleListShortcuts handles the `GET /shortcuts/` request and responds with a JSON formatted list
// of shortcut names.
func (s *Server) HandleListShortcuts(c web.C, w http.ResponseWriter, r *http.Request) {
	shortcuts, err := s.Store.List("/shortcuts/")
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(struct {
		Shortcuts []string `json:"shortcuts"`
	}{Shortcuts: shortcuts})
}

// HandleGetShortcut handles the `GET /shortcuts/:id` request and responds with the shortcut
// object in JSON format.
func (s *Server) HandleGetShortcut(c web.C, w http.ResponseWriter, r *http.Request) {
	shortcut := Shortcut{}
	err := s.Store.Get(fmt.Sprintf("/shortcuts/%s", c.URLParams["id"]), &shortcut)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(shortcut)
}

// HandlePutShortcut handles the `PUT /shortcuts/:id` request. It accepts a JSON formatted
// shortcut object in the request body and stores it.
func (s *Server) HandlePutShortcut(c web.C, w http.ResponseWriter, r *http.Request) {
	shortcut := Shortcut{}
	if err := json.NewDecoder(r.Body).Decode(&shortcut); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	shortcut.Name = c.URLParams["id"]

	err := s.Store.Put(fmt.Sprintf("/shortcuts/%s", c.URLParams["id"]), &shortcut)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteShortcut handles the `DELETE /shortcuts/:id` request.
func (s *Server) HandleDeleteShortcut(c web.C, w http.ResponseWriter, r *http.Request) {
	err := s.Store.Delete(fmt.Sprintf("/shortcuts/%s", c.URLParams["id"]))
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleIDPInitiated handles a request for an IDP initiated login flow. It looks up
// the specified shortcut, generates the appropriate SAML assertion and redirects the
// user via the HTTP-POST binding to the service providers ACS URL.
func (s *Server) HandleIDPInitiated(c web.C, w http.ResponseWriter, r *http.Request) {
	shortcutName := c.URLParams["shortcut"]
	shortcut := Shortcut{}
	if err := s.Store.Get(fmt.Sprintf("/shortcuts/%s", shortcutName), &shortcut); err != nil {
		log.Printf("ERROR: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	relayState := ""
	switch {
	case shortcut.RelayState != nil:
		relayState = *shortcut.RelayState
	case shortcut.URISuffixAsRelayState:
		relayState, _ = c.URLParams["*"]
	}

	s.idpConfigMu.RLock()
	defer s.idpConfigMu.RUnlock()
	s.IDP.ServeIDPInitiated(w, r, shortcut.ServiceProviderID, relayState)
}
