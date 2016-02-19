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
package reversetunnel

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/codahale/lunk"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/mailgun/oxy/forward"
	"golang.org/x/crypto/ssh"
)

// RemoteSite represents remote teleport site that can be accessed via
// teleport tunnel or directly by proxy
type RemoteSite interface {
	// ConnectToServer allows to SSH into remote teleport server
	ConnectToServer(addr, user string, auth []ssh.AuthMethod) (*ssh.Client, error)
	// DialServer dials teleport server and returns connection
	DialServer(addr string) (net.Conn, error)
	// GetLastConnected returns last time the remote site was seen connected
	GetLastConnected() time.Time
	// GetName returns site name (identified
	GetName() string
	// GetStatus returns status of this site (either offline or connected)
	GetStatus() string
	// GetClient returns client API to the remote site's auth server
	GetClient() *auth.Client
	// GetServers returns servers registered on this site
	GetServers() ([]services.Server, error)
	// GetHangoutInfo returns hangout info (used only if the site is in hangout mode
	GetHangoutInfo() *HangoutSiteInfo
}

type HangoutSiteInfo struct {
	HostKey  *services.CertAuthority
	OSUser   string
	AuthPort string
	NodePort string
}

// Server represents server connected to one or many remote sites
type Server interface {
	// GetSites returns a list of connected remote sites
	GetSites() []RemoteSite
	// GetSite returns remote site this node belongs to
	GetSite(name string) (RemoteSite, error)
	// FindSimilarSite returns site that matches domain name
	FindSimilarSite(name string) (RemoteSite, error)
	// Start starts server
	Start() error
	// Wait waits for server to close all outstanding operations
	Wait()
}

type server struct {
	sync.RWMutex

	ap              auth.AccessPoint
	hostCertChecker ssh.CertChecker
	userCertChecker ssh.CertChecker
	l               net.Listener
	srv             *sshutils.Server

	sites []*remoteSite
}

// NewServer returns an unstarted server
func NewServer(addr utils.NetAddr, hostSigners []ssh.Signer,
	ap auth.AccessPoint, limiter *limiter.Limiter) (Server, error) {
	srv := &server{
		sites: []*remoteSite{},
		ap:    ap,
	}
	s, err := sshutils.NewServer(
		addr,
		srv,
		hostSigners,
		sshutils.AuthMethods{
			PublicKey: srv.keyAuth,
		},
		limiter,
	)
	if err != nil {
		return nil, err
	}
	srv.hostCertChecker = ssh.CertChecker{IsAuthority: srv.isHostAuthority}
	srv.userCertChecker = ssh.CertChecker{IsAuthority: srv.isUserAuthority}
	srv.srv = s
	return srv, nil
}

func (s *server) Wait() {
	s.srv.Wait()
}

func (s *server) Addr() string {
	return s.srv.Addr()
}

func (s *server) Start() error {
	return s.srv.Start()
}

func (s *server) Close() error {
	return s.srv.Close()
}

func (s *server) HandleNewChan(sconn *ssh.ServerConn, nch ssh.NewChannel) {
	log.Infof("got new channel request: %v", nch.ChannelType())
	switch nch.ChannelType() {
	case chanHeartbeat:
		log.Infof("got heartbeat request from agent: %v", sconn)
		var site *remoteSite
		var err error

		switch sconn.Permissions.Extensions[extCertType] {
		case extCertTypeHost:
			site, err = s.upsertRegularSite(sconn)
		case extCertTypeUser:
			site, err = s.upsertHangoutSite(sconn)
		default:
			err = trace.Wrap(
				teleport.BadParameter(
					"certType", "can't retrieve certificate type"))
		}
		if err != nil {
			log.Errorf("failed to upsert site: %v", err)
			nch.Reject(ssh.ConnectionFailed, "failed to upsert site")
			return
		}

		ch, req, err := nch.Accept()
		if err != nil {
			log.Errorf("failed to accept channel: %v", err)
			sconn.Close()
			return
		}
		go site.handleHeartbeat(ch, req)
	}
}

// isHostAuthority is called during checking the client key, to see if the signing
// key is the real host CA authority key.
func (s *server) isHostAuthority(auth ssh.PublicKey) bool {
	keys, err := s.getTrustedCAKeys(services.HostCA)
	if err != nil {
		log.Errorf("failed to retrieve trusted keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}
	return false
}

// isUserAuthority is called during checking the client key, to see if the signing
// key is the real user CA authority key.
func (s *server) isUserAuthority(auth ssh.PublicKey) bool {
	keys, err := s.getTrustedCAKeys(services.UserCA)
	if err != nil {
		log.Errorf("failed to retrieve trusted keys, err: %v", err)
		return false
	}
	for _, k := range keys {
		if sshutils.KeysEqual(k, auth) {
			return true
		}
	}
	return false
}

func (s *server) getTrustedCAKeys(CertType services.CertAuthType) ([]ssh.PublicKey, error) {
	cas, err := s.ap.GetCertAuthorities(CertType)
	if err != nil {
		return nil, err
	}
	out := []ssh.PublicKey{}
	for _, ca := range cas {
		checkers, err := ca.Checkers()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, checkers...)
	}
	return out, nil
}

func (s *server) checkTrustedKey(CertType services.CertAuthType, domainName string, key ssh.PublicKey) error {
	cas, err := s.ap.GetCertAuthorities(CertType)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, ca := range cas {
		if ca.DomainName != domainName {
			continue
		}
		checkers, err := ca.Checkers()
		if err != nil {
			return trace.Wrap(err)
		}
		for _, checker := range checkers {
			if sshutils.KeysEqual(key, checker) {
				return nil
			}
		}
	}
	return trace.Wrap(&teleport.NotFoundError{
		Message: fmt.Sprintf("authority domain %v not found or has no mathching keys", domainName)})
}

func (s *server) keyAuth(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	logger := log.WithFields(log.Fields{
		"remote": conn.RemoteAddr(),
		"user":   conn.User(),
	})

	logger.Infof("auth attempt with key %v", key.Type())

	cert, ok := key.(*ssh.Certificate)
	if !ok {
		logger.Warningf("server doesn't support provided key type")
		return nil, trace.Wrap(
			teleport.BadParameter("key", "server doesn't support provided key type"))
	}

	switch cert.CertType {
	case ssh.HostCert:
		authDomain, ok := cert.Extensions[utils.CertExtensionAuthority]
		if !ok || authDomain == "" {
			err := trace.Wrap(teleport.BadParameter("domainName", "missing authority domain"))
			logger.Warningf("failed authenticate host, err: %v", err)
			return nil, err
		}
		err := s.hostCertChecker.CheckHostKey(conn.User(), conn.RemoteAddr(), key)
		if err != nil {
			logger.Warningf("failed authenticate host, err: %v", err)
			return nil, trace.Wrap(err)
		}
		if err := s.hostCertChecker.CheckCert(conn.User(), cert); err != nil {
			logger.Warningf("failed to authenticate host err: %v", err)
			return nil, trace.Wrap(err)
		}
		// this fixes possible injection attack
		// when we have 2 trusted remote sites, and one can simply
		// pose as another. so we have to check that authority
		// matches by some other way (in absence of x509 chains)
		if err := s.checkTrustedKey(services.HostCA, authDomain, cert.SignatureKey); err != nil {
			logger.Warningf("this claims to be signed as authDomain %v, but no matching signing keys found")
			return nil, trace.Wrap(err)
		}
		return &ssh.Permissions{
			Extensions: map[string]string{
				extHost:      conn.User(),
				extCertType:  extCertTypeHost,
				extAuthority: authDomain,
			},
		}, nil
	case ssh.UserCert:
		_, err := s.userCertChecker.Authenticate(conn, key)
		if err != nil {
			logger.Warningf("failed to authenticate user, err: %v", err)
			return nil, err
		}

		if err := s.userCertChecker.CheckCert(conn.User(), cert); err != nil {
			logger.Warningf("failed to authenticate user err: %v", err)
			return nil, trace.Wrap(err)
		}

		return &ssh.Permissions{
			Extensions: map[string]string{
				extHost:     conn.User(),
				extCertType: extCertTypeUser,
			},
		}, nil
	default:
		return nil, trace.Wrap(
			teleport.BadParameter("cert",
				fmt.Sprintf("unsupported cert type: %v", cert.CertType)))
	}
}

func (s *server) upsertRegularSite(c *ssh.ServerConn) (*remoteSite, error) {

	domainName := c.Permissions.Extensions[extAuthority]
	if !cstrings.IsValidDomainName(domainName) {
		return nil, trace.Wrap(teleport.BadParameter(
			"authDomain", fmt.Sprintf("'%v' is a bad domain name", domainName)))
	}
	var site *remoteSite
	for _, st := range s.sites {
		if st.domainName == domainName {
			site = st
			break
		}
	}
	log.Infof("found authority domain: %v", domainName)

	s.Lock()
	defer s.Unlock()

	if site != nil {
		if err := site.init(c); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		site = &remoteSite{srv: s, domainName: c.User()}
		if err := site.init(c); err != nil {
			return nil, trace.Wrap(err)
		}
		s.sites = append(s.sites, site)
	}
	return site, nil
}

func (s *server) upsertHangoutSite(c ssh.Conn) (*remoteSite, error) {
	s.Lock()
	defer s.Unlock()

	hangoutID := c.User()
	for _, st := range s.sites {
		if st.domainName == hangoutID {
			return nil, trace.Errorf("Hangout ID is already used")
		}
	}

	site := &remoteSite{srv: s, domainName: hangoutID}
	err := site.init(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hangoutCertAuthorities, err := site.clt.GetCertAuthorities(services.HostCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(hangoutCertAuthorities) != 1 {
		return nil, trace.Errorf("Can't retrieve hangout Certificate Authority")
	}
	site.hangoutInfo = &HangoutSiteInfo{
		HostKey: hangoutCertAuthorities[0],
	}

	proxyUserCertAuthorities, err := s.ap.GetCertAuthorities(services.UserCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, ca := range proxyUserCertAuthorities {
		err := site.clt.UpsertCertAuthority(*ca, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// receiving hangoutInfo using sessions just as storage
	sess, err := site.clt.GetSessions()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sess) != 1 {
		return nil, trace.Wrap(&teleport.NotFoundError{
			Message: fmt.Sprintf("hangout %v not found", hangoutID),
		})
	}
	hangoutInfo, err := utils.UnmarshalHangoutInfo(sess[0].ID)
	if err != nil {
		return nil, err
	}
	site.domainName = hangoutInfo.HangoutID

	site.hangoutInfo.OSUser = hangoutInfo.OSUser
	site.hangoutInfo.AuthPort = hangoutInfo.AuthPort
	site.hangoutInfo.NodePort = hangoutInfo.NodePort

	s.sites = append(s.sites, site)
	return site, nil
}

func (s *server) GetSites() []RemoteSite {
	s.RLock()
	defer s.RUnlock()
	out := make([]RemoteSite, len(s.sites))
	for i := range s.sites {
		out[i] = s.sites[i]
	}
	return out
}

func (s *server) GetSite(domainName string) (RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()
	for i := range s.sites {
		if s.sites[i].domainName == domainName {
			return s.sites[i], nil
		}
	}
	return nil, fmt.Errorf("site %v not found", domainName)
}

// FindSimilarSite finds the site that is the most similar to domain.
// Returns nil if no sites with such domain name.
func (s *server) FindSimilarSite(domainName string) (RemoteSite, error) {
	s.RLock()
	defer s.RUnlock()

	result := -1
	resultSimilarity := 1

	domainName1 := strings.Split(domainName, ".")
	log.Infof("Find matching domain: %v", domainName)

	for i, site := range s.sites {
		log.Infof(site.domainName)
		domainName2 := strings.Split(site.domainName, ".")
		similarity := 0
		for j := 1; (j <= len(domainName1)) && (j <= len(domainName2)); j++ {
			if domainName1[len(domainName1)-j] != domainName2[len(domainName2)-j] {
				break
			}
			similarity++
		}
		if (similarity > resultSimilarity) || (result == -1) {
			result = i
			resultSimilarity = similarity
		}
	}

	if result != -1 {
		return s.sites[result], nil
	}
	return nil, trace.Wrap(&teleport.NotFoundError{
		Message: fmt.Sprintf("no site matching '%v' found", domainName),
	})
}

type remoteSite struct {
	domainName string
	conn       ssh.Conn
	lastActive time.Time
	srv        *server
	clt        *auth.Client

	hangoutInfo *HangoutSiteInfo
}

func (s *remoteSite) GetClient() *auth.Client {
	return s.clt
}

func (s *remoteSite) GetEvents(filter events.Filter) ([]lunk.Entry, error) {
	return s.clt.GetEvents(filter)
}

func (s *remoteSite) String() string {
	return fmt.Sprintf("remoteSite(%v)", s.domainName)
}

func (s *remoteSite) init(c ssh.Conn) error {
	if s.conn != nil {
		log.Infof("%v found site, closing previous connection", s)
		s.conn.Close()
	}
	s.conn = c
	tr := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			ch, _, err := s.conn.OpenChannel(chanAccessPoint, nil)
			if err != nil {
				log.Errorf("remoteSite:authProxy %v", err)
				return nil, err
			}
			return utils.NewChConn(s.conn, ch), nil
		},
	}
	clt, err := auth.NewClient(
		"http://stub:0",
		roundtrip.HTTPClient(&http.Client{
			Transport: tr,
		}))
	if err != nil {
		return err
	}
	s.clt = clt
	return nil
}

func (s *remoteSite) GetStatus() string {
	diff := time.Now().Sub(s.lastActive)
	if diff > 2*heartbeatPeriod {
		return RemoteSiteStatusOffline
	}
	return RemoteSiteStatusOnline
}

func (s *remoteSite) handleHeartbeat(ch ssh.Channel, reqC <-chan *ssh.Request) {
	go func() {
		for {
			req := <-reqC
			if req == nil {
				log.Infof("agent disconnected")
				return
			}
			log.Debugf("%v -> ping", s)
			s.lastActive = time.Now()
		}
	}()
}

func (s *remoteSite) GetName() string {
	return s.domainName
}

func (s *remoteSite) GetLastConnected() time.Time {
	return s.lastActive
}

func (s *remoteSite) ConnectToServer(server, user string, auth []ssh.AuthMethod) (*ssh.Client, error) {
	ch, _, err := s.conn.OpenChannel(chanTransport, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// ask remote channel to dial
	dialed, err := ch.SendRequest(chanTransportDialReq, true, []byte(server))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !dialed {
		return nil, trace.Errorf("remote server %v is not available", server)
	}
	transportConn := utils.NewChConn(s.conn, ch)
	conn, chans, reqs, err := ssh.NewClientConn(
		transportConn, server,
		&ssh.ClientConfig{
			User: user,
			Auth: auth,
		})
	if err != nil {
		log.Errorf("remoteSite:connectToServer %v", err)
		return nil, err
	}
	return ssh.NewClient(conn, chans, reqs), nil
}

func (s *remoteSite) DialServer(server string) (net.Conn, error) {
	serverIsKnown := false
	knownServers, err := s.GetServers()

	for _, srv := range knownServers {
		_, port, err := net.SplitHostPort(srv.Addr)
		if err != nil {
			log.Errorf("server %v(%v) has incorrect address format (%v)",
				srv.Addr, srv.Hostname, err.Error())
		} else {
			if (len(srv.Hostname) != 0) && (len(port) != 0) && (server == srv.Hostname+":"+port || server == srv.Addr) {
				serverIsKnown = true
			}
		}
	}
	if !serverIsKnown {
		return nil, trace.Errorf("can't dial server %v, server is unknown", server)
	}

	ch, _, err := s.conn.OpenChannel(chanTransport, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// ask remote channel to dial
	dialed, err := ch.SendRequest(chanTransportDialReq, true, []byte(server))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !dialed {
		return nil, trace.Errorf("remote server %v is not available", server)
	}
	return utils.NewChConn(s.conn, ch), nil
}

func (s *remoteSite) GetServers() ([]services.Server, error) {
	return s.clt.GetServers()
}

func (s *remoteSite) handleAuthProxy(w http.ResponseWriter, r *http.Request) {
	tr := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			ch, _, err := s.conn.OpenChannel(chanAccessPoint, nil)
			if err != nil {
				log.Errorf("remoteSite:authProxy %v", err)
				return nil, err
			}
			return utils.NewChConn(s.conn, ch), nil
		},
	}

	fwd, err := forward.New(forward.RoundTripper(tr), forward.Logger(log.StandardLogger()))
	if err != nil {
		log.Errorf("write: %v", err)
		roundtrip.ReplyJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	r.URL.Scheme = "http"
	r.URL.Host = "stub"
	fwd.ServeHTTP(w, r)
}

func (s *remoteSite) GetHangoutInfo() *HangoutSiteInfo {
	return s.hangoutInfo
}

const (
	extHost         = "host@teleport"
	extCertType     = "certtype@teleport"
	extAuthority    = "auth@teleport"
	extCertTypeHost = "host"
	extCertTypeUser = "user"
)
