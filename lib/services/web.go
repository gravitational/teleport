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
package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gokyle/hotp"

	"github.com/gravitational/log"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
)

type WebService struct {
	backend     backend.Backend
	SignupMutex *sync.Mutex
}

func NewWebService(backend backend.Backend) *WebService {
	return &WebService{
		backend:     backend,
		SignupMutex: &sync.Mutex{},
	}
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

func (s *WebService) UpsertHOTP(user string, otp *hotp.HOTP) error {
	bytes, err := hotp.Marshal(otp)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.backend.UpsertVal([]string{"web", "users", user},
		"hotp", bytes, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *WebService) GetHOTP(user string) (*hotp.HOTP, error) {
	bytes, err := s.backend.GetVal([]string{"web", "users", user},
		"hotp")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	otp, err := hotp.Unmarshal(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return otp, nil
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

func (s *WebService) UpsertPassword(user string,
	password []byte) (hotpURL string, hotpQR []byte, err error) {

	if err := verifyPassword(password); err != nil {
		return "", nil, err
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}

	otp, err := hotp.GenerateHOTP(HOTPTokenDigits, false)
	if err != nil {
		return "", nil, err
	}
	hotpQR, err = otp.QR(user)
	if err != nil {
		return "", nil, err
	}
	hotpURL = otp.URL(user)
	if err != nil {
		return "", nil, err
	}
	otp.Increment()

	err = s.UpsertPasswordHash(user, hash)
	if err != nil {
		return "", nil, err
	}
	err = s.UpsertHOTP(user, otp)
	if err != nil {
		return "", nil, err
	}

	return hotpURL, hotpQR, nil

}

func (s *WebService) CheckPassword(user string, password []byte, hotpToken string) error {
	if err := verifyPassword(password); err != nil {
		return trace.Wrap(err)
	}
	hash, err := s.GetPasswordHash(user)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		return &teleport.BadParameterError{Err: "passwords do not match", Param: "password"}
	}

	otp, err := s.GetHOTP(user)
	if err != nil {
		return trace.Wrap(err)
	}

	if !otp.Scan(hotpToken, 4) {
		return &teleport.BadParameterError{Err: "tokens do not match", Param: "token"}
	}

	if err := s.UpsertHOTP(user, otp); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// TO DO: not very good
func (s *WebService) CheckPasswordWOToken(user string, password []byte) error {
	if err := verifyPassword(password); err != nil {
		return trace.Wrap(err)
	}
	hash, err := s.GetPasswordHash(user)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, password); err != nil {
		return &teleport.BadParameterError{Err: "passwords do not match"}
	}

	return nil
}

// make sure password satisfies our requirements (relaxed),
// mostly to avoid putting garbage in
func verifyPassword(password []byte) error {
	if len(password) < MinPasswordLength {
		return &teleport.BadParameterError{
			Param: "password",
			Err: fmt.Sprintf(
				"password is too short, min length is %v", MinPasswordLength),
		}
	}
	if len(password) > MaxPasswordLength {
		return &teleport.BadParameterError{
			Param: "password",
			Err: fmt.Sprintf(
				"password is too long, max length is %v", MaxPasswordLength),
		}
	}
	return nil
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

type SignupToken struct {
	Token           string
	User            string
	Hotp            []byte
	HotpFirstValues []string
	HotpQR          []byte
}

func (s *WebService) UpsertSignupToken(token string, tokenData SignupToken, ttl time.Duration) error {
	out, err := json.Marshal(tokenData)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.backend.UpsertVal([]string{"addusertokens"}, token, out, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil

}
func (s *WebService) GetSignupToken(token string) (tokenData SignupToken,
	ttl time.Duration, e error) {

	out, ttl, err := s.backend.GetValAndTTL([]string{"addusertokens"}, token)
	if err != nil {
		return SignupToken{}, 0, trace.Wrap(err)
	}
	var data SignupToken
	err = json.Unmarshal(out, &data)
	if err != nil {
		return SignupToken{}, 0, trace.Wrap(err)
	}

	return data, ttl, nil
}
func (s *WebService) DeleteSignupToken(token string) error {
	err := s.backend.DeleteKey([]string{"addusertokens"}, token)
	return err
}

const (
	MinPasswordLength = 6
	MaxPasswordLength = 128
	HOTPTokenDigits   = 6 //number of digits in each token
)
