package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/backend"
)

type WebService struct {
	backend backend.Backend
}

func NewWebService(backend backend.Backend) *WebService {
	return &WebService{backend}
}

// UpsertPasswordHash upserts user password hash
func (s *WebService) UpsertPasswordHash(user string, hash []byte) error {
	err := s.backend.UpsertVal([]string{"web", "users", user},
		"pwd", hash, 0)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return err
}

// GetPasswordHash returns the password hash for a given user
func (s *WebService) GetPasswordHash(user string) ([]byte, error) {
	hash, err := s.backend.GetVal([]string{"web", "users", user}, "pwd")
	if err != nil {
		return nil, err
	}
	return hash, err
}

// UpsertSession
func (s *WebService) UpsertWebSession(user, sid string,
	session WebSession, ttl time.Duration) error {

	bytes, err := json.Marshal(session)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal([]string{"web", "users", user, "sessions"},
		sid, bytes, ttl)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return err

}

// GetWebSession
func (s *WebService) GetWebSession(user, sid string) (*WebSession, error) {
	val, err := s.backend.GetVal(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	if err != nil {
		return nil, err
	}

	var session WebSession
	err = json.Unmarshal(val, &session)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &session, nil
}

// GetWebSessionsKeys
func (s *WebService) GetWebSessionsKeys(user string) ([]AuthorizedKey, error) {
	keys, err := s.backend.GetKeys([]string{"web", "users", user, "sessions"})
	if err != nil {
		return nil, err
	}

	values := make([]AuthorizedKey, len(keys))
	for i, key := range keys {
		session, err := s.GetWebSession(user, key)
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		values[i].Value = session.Pub
	}
	return values, nil
}

// DeleteWebSession
func (s *WebService) DeleteWebSession(user, sid string) error {
	err := s.backend.DeleteKey(
		[]string{"web", "users", user, "sessions"},
		sid,
	)
	return err
}

func (s *WebService) UpsertWebTun(tun WebTun, ttl time.Duration) error {
	if tun.Prefix == "" {
		log.Errorf("Missing parameter 'Prefix'")
		return fmt.Errorf("Missing parameter 'Prefix'")
	}

	bytes, err := json.Marshal(tun)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal([]string{"web", "tunnels"},
		tun.Prefix, bytes, ttl)
	if err != nil {
		log.Errorf(err.Error())
		return trace.Wrap(err)
	}
	return nil
}

func (s *WebService) DeleteWebTun(prefix string) error {
	err := s.backend.DeleteKey(
		[]string{"web", "tunnels"},
		prefix,
	)
	return err
}
func (s *WebService) GetWebTun(prefix string) (*WebTun, error) {
	val, err := s.backend.GetVal(
		[]string{"web", "tunnels"},
		prefix,
	)
	if err != nil {
		return nil, err
	}

	var tun WebTun
	err = json.Unmarshal(val, &tun)
	if err != nil {
		log.Errorf(err.Error())
		return nil, trace.Wrap(err)
	}

	return &tun, nil
}
func (s *WebService) GetWebTuns() ([]WebTun, error) {
	keys, err := s.backend.GetKeys([]string{"web", "tunnels"})
	if err != nil {
		return nil, err
	}

	tuns := make([]WebTun, len(keys))
	for i, key := range keys {
		tun, err := s.GetWebTun(key)
		if err != nil {
			log.Errorf(err.Error())
			return nil, trace.Wrap(err)
		}
		tuns[i] = *tun
	}
	return tuns, nil
}

type WebSession struct {
	Pub  []byte `json:"pub"`
	Priv []byte `json:"priv"`
}

// WebTun is a web tunnel, the SSH tunnel
// created by the SSH server to a remote web server
type WebTun struct {
	// Prefix is a domain prefix that will be used
	// to serve this tunnel
	Prefix string `json:"prefix"`
	// ProxyAddr is the address of the SSH server
	// that will be acting as a SSH proxy
	ProxyAddr string `json:"proxy"`
	// TargetAddr is the target http address of the server
	TargetAddr string `json:"target"`
}

func NewWebTun(prefix, proxyAddr, targetAddr string) (*WebTun, error) {
	if prefix == "" {
		return nil, &teleport.MissingParameterError{Param: "prefix"}
	}
	if targetAddr == "" {
		return nil, &teleport.MissingParameterError{Param: "target"}
	}
	if proxyAddr == "" {
		return nil, &teleport.MissingParameterError{Param: "proxy"}
	}
	if _, err := url.ParseRequestURI(targetAddr); err != nil {
		return nil, &teleport.BadParameterError{Param: "target", Err: err.Error()}
	}
	return &WebTun{Prefix: prefix, ProxyAddr: proxyAddr, TargetAddr: targetAddr}, nil
}
