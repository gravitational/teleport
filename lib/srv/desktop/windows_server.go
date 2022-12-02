/*
Copyright 2021 Gravitational, Inc.

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

package desktop

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/filesessions"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// dnsDialTimeout is the timeout for dialing the LDAP server
	// when resolving Windows Desktop hostnames
	dnsDialTimeout = 5 * time.Second

	// ldapDialTimeout is the timeout for dialing the LDAP server
	// when making an initial connection
	ldapDialTimeout = 5 * time.Second

	// ldapRequestTimeout is the timeout for making LDAP requests.
	// It is larger than the dial timeout because LDAP queries in large
	// Active Directory environments may take longer to complete.
	ldapRequestTimeout = 20 * time.Second

	// windowsDesktopCertTTL is the TTL for Teleport-issued Windows Certificates.
	// Certificates are requested on each connection attempt, so the TTL is
	// deliberately set to a small value to give enough time to establish a
	// single desktop session.
	windowsDesktopCertTTL = 5 * time.Minute

	// windowsDesktopServiceCertTTL is the TTL for certificates issued to the
	// Windows Desktop Service in order to authenticate with the LDAP server.
	// It is set longer than the Windows certificates for users because it is
	// not used for interactive login and is only used when issuing certs for
	// a restrictive service account.
	windowsDesktopServiceCertTTL = 8 * time.Hour

	// windowsDesktopServiceCertRetryInterval indicates how often to retry
	// issuing an LDAP certificate if the operation fails.
	windowsDesktopServiceCertRetryInterval = 10 * time.Minute
)

// WindowsService implements the RDP-based Windows desktop access service.
//
// This service accepts mTLS connections from the proxy, establishes RDP
// connections to Windows hosts and translates RDP into Teleport's desktop
// protocol.
type WindowsService struct {
	cfg        WindowsServiceConfig
	middleware *auth.Middleware

	lc *ldapClient

	mu              sync.Mutex // mu protects the fields that follow
	ldapInitialized bool
	ldapCertRenew   *time.Timer

	streamer libevents.Streamer

	// lastDisoveryResults stores the results of the most recent LDAP search
	// when desktop discovery is enabled.
	// no synchronization is necessary because this is only read/written from
	// the reconciler goroutine.
	lastDiscoveryResults types.ResourcesWithLabelsMap

	// Windows hosts discovered via LDAP likely won't resolve with the
	// default DNS resolver, so we need a custom resolver that will
	// query the domain controller.
	dnsResolver *net.Resolver

	// clusterName is the cached local cluster name, to avoid calling
	// cfg.AccessPoint.GetClusterName multiple times.
	clusterName string

	closeCtx context.Context
	close    func()
}

// WindowsServiceConfig contains all necessary configuration values for a
// WindowsService.
type WindowsServiceConfig struct {
	// Log is the logging sink for the service.
	Log logrus.FieldLogger
	// Clock provides current time.
	Clock   clockwork.Clock
	DataDir string
	// Authorizer is used to authorize requests.
	Authorizer auth.Authorizer
	// LockWatcher is used to monitor for new locks.
	LockWatcher *services.LockWatcher
	// Emitter emits audit log events.
	Emitter events.Emitter
	// TLS is the TLS server configuration.
	TLS *tls.Config
	// AccessPoint is the Auth API client (with caching).
	AccessPoint auth.WindowsDesktopAccessPoint
	// AuthClient is the Auth API client (without caching).
	AuthClient auth.ClientI
	// ConnLimiter limits the number of active connections per client IP.
	ConnLimiter *limiter.ConnectionsLimiter
	// Heartbeat contains configuration for service heartbeats.
	Heartbeat HeartbeatConfig
	// HostLabelsFn gets labels that should be applied to a Windows host.
	HostLabelsFn func(host string) map[string]string
	// LDAPConfig contains parameters for connecting to an LDAP server.
	LDAPConfig
	// DiscoveryBaseDN is the base DN for searching for Windows Desktops.
	// Desktop discovery is disabled if this field is empty.
	DiscoveryBaseDN string
	// DiscoveryLDAPFilters are additional LDAP filters for searching for
	// Windows Desktops. If multiple filters are specified, they are ANDed
	// together into a single search.
	DiscoveryLDAPFilters []string
	// DiscoveryLDAPAttributeLabels are optional LDAP attributes to convert
	// into Teleport labels.
	DiscoveryLDAPAttributeLabels []string
	// Hostname of the windows desktop service
	Hostname string
}

// LDAPConfig contains parameters for connecting to an LDAP server.
type LDAPConfig struct {
	// Addr is the LDAP server address in the form host:port.
	// Standard port is 636 for LDAPS.
	Addr string
	// Domain is an Active Directory domain name, like "example.com".
	Domain string
	// Username is an LDAP username, like "EXAMPLE\Administrator", where
	// "EXAMPLE" is the NetBIOS version of Domain.
	Username string
	// InsecureSkipVerify decides whether whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool
	// CA is an optional CA cert to be used for verification if InsecureSkipVerify is set to false.
	CA *x509.Certificate
}

func (cfg LDAPConfig) check() error {
	if cfg.Addr == "" {
		return trace.BadParameter("missing Addr in LDAPConfig")
	}
	if cfg.Domain == "" {
		return trace.BadParameter("missing Domain in LDAPConfig")
	}
	if cfg.Username == "" {
		return trace.BadParameter("missing Username in LDAPConfig")
	}
	return nil
}

func (cfg LDAPConfig) domainDN() string {
	var sb strings.Builder
	parts := strings.Split(cfg.Domain, ".")
	for _, p := range parts {
		if sb.Len() > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("DC=")
		sb.WriteString(p)
	}
	return sb.String()
}

// HeartbeatConfig contains the configuration for service heartbeats.
type HeartbeatConfig struct {
	// HostUUID is the UUID of the host that this service runs on. Used as the
	// name of the created API object.
	HostUUID string
	// PublicAddr is the public address of this service.
	PublicAddr string
	// OnHeartbeat is called after each heartbeat attempt.
	OnHeartbeat func(error)
	// StaticHosts is an optional list of static Windows hosts to register.
	StaticHosts []utils.NetAddr
}

func (cfg *WindowsServiceConfig) checkAndSetDiscoveryDefaults() error {
	switch {
	case cfg.DiscoveryBaseDN == types.Wildcard:
		cfg.DiscoveryBaseDN = cfg.domainDN()
	case len(cfg.DiscoveryBaseDN) > 0:
		if _, err := ldap.ParseDN(cfg.DiscoveryBaseDN); err != nil {
			return trace.BadParameter("WindowsServiceConfig contains an invalid base_dn: %v", err)
		}
	}

	for _, filter := range cfg.DiscoveryLDAPFilters {
		if _, err := ldap.CompileFilter(filter); err != nil {
			return trace.BadParameter("WindowsServiceConfig contains an invalid LDAP filter %q: %v", filter, err)
		}
	}

	return nil
}

func (cfg *WindowsServiceConfig) CheckAndSetDefaults() error {
	if cfg.Log == nil {
		cfg.Log = logrus.New().WithField(trace.Component, teleport.ComponentWindowsDesktop)
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Authorizer == nil {
		return trace.BadParameter("WindowsServiceConfig is missing Authorizer")
	}
	if cfg.LockWatcher == nil {
		return trace.BadParameter("WindowsServiceConfig is missing LockWatcher")
	}
	if cfg.Emitter == nil {
		return trace.BadParameter("WindowsServiceConfig is missing Emitter")
	}
	if cfg.TLS == nil {
		return trace.BadParameter("WindowsServiceConfig is missing TLS")
	}
	if cfg.AccessPoint == nil {
		return trace.BadParameter("WindowsServiceConfig is missing AccessPoint")
	}
	if cfg.AuthClient == nil {
		return trace.BadParameter("WindowsServiceConfig is missing AuthClient")
	}
	if cfg.ConnLimiter == nil {
		return trace.BadParameter("WindowsServiceConfig is missing ConnLimiter")
	}
	if err := cfg.Heartbeat.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if err := cfg.LDAPConfig.check(); err != nil {
		return trace.Wrap(err)
	}
	if err := cfg.checkAndSetDiscoveryDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (cfg *HeartbeatConfig) CheckAndSetDefaults() error {
	if cfg.HostUUID == "" {
		return trace.BadParameter("HeartbeatConfig is missing HostUUID")
	}
	if cfg.PublicAddr == "" {
		return trace.BadParameter("HeartbeatConfig is missing PublicAddr")
	}
	if cfg.OnHeartbeat == nil {
		return trace.BadParameter("HeartbeatConfig is missing OnHeartbeat")
	}
	return nil
}

// NewWindowsService initializes a new WindowsService.
//
// To start serving connections, call Serve.
// When done serving connections, call Close.
func NewWindowsService(cfg WindowsServiceConfig) (*WindowsService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// It's possible to provide a CA certificate for the LDAP server
	// and to skip TLS valdiation, though this may be an error, so try
	// to warn the user.
	// (You may need this configuration in order to use certificates to
	// authenticate with LDAP when the LDAP server name is not correct
	// in the certificate).
	if cfg.LDAPConfig.CA != nil && cfg.LDAPConfig.InsecureSkipVerify {
		cfg.Log.Warn("LDAP configuration specifies both der_ca_file and insecure_skip_verify." +
			"TLS connections to the LDAP server will not be verified. If this is intentional, disregard this warning.")
	}

	clusterName, err := cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "fetching cluster name")
	}

	// Here we assume the LDAP server is an Active Directory Domain Controller,
	// which means it should also be a DNS server that can resolve Windows hosts.
	dnsServer, _, err := net.SplitHostPort(cfg.LDAPConfig.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsAddr := net.JoinHostPort(dnsServer, "53")
	cfg.Log.Debugln("DNS lookups will be performed against", dnsAddr)

	ctx, close := context.WithCancel(context.Background())
	s := &WindowsService{
		cfg: cfg,
		middleware: &auth.Middleware{
			AccessPoint:   cfg.AccessPoint,
			AcceptedUsage: []string{teleport.UsageWindowsDesktopOnly},
		},
		dnsResolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Ignore the address provided, and always explicitly dial
				// the domain controller.
				d := net.Dialer{Timeout: dnsDialTimeout}
				return d.DialContext(ctx, network, dnsAddr)
			},
		},
		lc:          &ldapClient{cfg: cfg.LDAPConfig},
		clusterName: clusterName.GetClusterName(),
		closeCtx:    ctx,
		close:       close,
	}

	// initialize LDAP - if this fails it will automatically schedule a retry.
	// we don't want to return an error in this case, because failure to start
	// the service brings down the entire Teleport process
	if err := s.initializeLDAP(); err != nil {
		s.cfg.Log.WithError(err).Error("initializing LDAP client, will retry")
	}

	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()

	recConfig, err := s.cfg.AccessPoint.GetSessionRecordingConfig(s.closeCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	streamer, err := s.newStreamer(s.closeCtx, recConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.streamer = streamer

	if err := s.startServiceHeartbeat(); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.startStaticHostHeartbeats(); err != nil {
		return nil, trace.Wrap(err)
	}

	if len(s.cfg.DiscoveryBaseDN) > 0 {
		if err := s.startDesktopDiscovery(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else if len(s.cfg.Heartbeat.StaticHosts) == 0 {
		s.cfg.Log.Warnln("desktop discovery via LDAP is disabled, and no hosts are defined in the configuration; there will be no Windows desktops available to connect")
	} else {
		s.cfg.Log.Infoln("desktop discovery via LDAP is disabled, set 'base_dn' to enable")
	}

	ok = true
	return s, nil
}

func (s *WindowsService) newStreamWriter(record bool, sessionID string) (libevents.StreamWriter, error) {
	return libevents.NewAuditWriter(libevents.AuditWriterConfig{
		Component:    teleport.ComponentWindowsDesktop,
		Namespace:    apidefaults.Namespace,
		Context:      s.closeCtx,
		Clock:        s.cfg.Clock,
		ClusterName:  s.clusterName,
		SessionID:    session.ID(sessionID),
		Streamer:     s.streamer,
		ServerID:     s.cfg.Heartbeat.HostUUID,
		RecordOutput: record,
	})
}

// newStreamer creates a streamer (sync or async) based on the cluster configuration.
// Synchronous streamers send events directly to the auth server, and blocks if the server
// cannot keep up. Asynchronous streamers buffers the events to disk and uploads them later.
func (s *WindowsService) newStreamer(ctx context.Context, recConfig types.SessionRecordingConfig) (libevents.Streamer, error) {
	if services.IsRecordSync(recConfig.GetMode()) {
		s.cfg.Log.Debugf("using sync streamer (for mode %v)", recConfig.GetMode())
		return s.cfg.AuthClient, nil
	}
	s.cfg.Log.Debugf("using async streamer (for mode %v)", recConfig.GetMode())
	uploadDir := filepath.Join(s.cfg.DataDir, teleport.LogsDir, teleport.ComponentUpload,
		libevents.StreamingSessionsDir, apidefaults.Namespace)
	fileStreamer, err := filesessions.NewStreamer(uploadDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return libevents.NewTeeStreamer(fileStreamer, s.cfg.Emitter), nil
}

func (s *WindowsService) tlsConfigForLDAP() (*tls.Config, error) {
	// trim NETBIOS name from username
	user := s.cfg.Username
	if i := strings.LastIndex(s.cfg.Username, `\`); i != -1 {
		user = user[i+1:]
	}

	certDER, keyDER, err := s.generateCredentials(s.closeCtx, user, s.cfg.Domain, windowsDesktopServiceCertTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing cert DER")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyDER)
	if err != nil {
		return nil, trace.Wrap(err, "parsing key DER")
	}

	tc := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{cert.Raw},
				PrivateKey:  key,
			},
		},
		InsecureSkipVerify: s.cfg.InsecureSkipVerify,
	}

	if s.cfg.CA != nil {
		pool := x509.NewCertPool()
		pool.AddCert(s.cfg.CA)
		tc.RootCAs = pool
	}

	return tc, nil
}

// initializeLDAP requests a TLS certificate from the auth server to be used for
// authenticating with the LDAP server. If the certificate is obtained, and
// authentication with the LDAP server succeeds, it schedules a renewal to take
// place before the certificate expires. If we are unable to obtain a certificate
// and authenticate with the LDAP server, then the operation will be automatically
// retried.
func (s *WindowsService) initializeLDAP() error {
	tc, err := s.tlsConfigForLDAP()
	if trace.IsAccessDenied(err) && modules.GetModules().BuildType() == modules.BuildEnterprise {
		s.cfg.Log.Warn("Could not generate certificate for LDAPS. Ensure that the auth server is licensed for desktop access.")
	}
	if err != nil {
		s.mu.Lock()
		s.ldapInitialized = false
		// in the case where we're not licensed for desktop access, we retry less frequently,
		// since this is likely not an intermittent error that will resolve itself quickly
		s.scheduleNextLDAPCertRenewalLocked(windowsDesktopServiceCertRetryInterval * 3)
		s.mu.Unlock()
		return trace.Wrap(err)
	}

	conn, err := ldap.DialURL("ldaps://"+s.cfg.Addr,
		ldap.DialWithTLSDialer(tc, &net.Dialer{Timeout: ldapDialTimeout}))
	if err != nil {
		s.mu.Lock()
		s.ldapInitialized = false
		s.scheduleNextLDAPCertRenewalLocked(windowsDesktopServiceCertRetryInterval)
		s.mu.Unlock()
		return trace.Wrap(err, "dial")
	}

	conn.SetTimeout(ldapRequestTimeout)
	s.lc.setClient(conn)

	// Note: admin still needs to import our CA into the Group Policy following
	// https://docs.vmware.com/en/VMware-Horizon-7/7.13/horizon-installation/GUID-7966AE16-D98F-430E-A916-391E8EAAFE18.html
	//
	// We can find the group policy object via LDAP, but it only contains an
	// SMB file path with the actual policy. See
	// https://en.wikipedia.org/wiki/Group_Policy
	//
	// In theory, we could update the policy file(s) over SMB following
	// https://docs.microsoft.com/en-us/previous-versions/windows/desktop/policy/registry-policy-file-format,
	// but I'm leaving this for later.
	//
	if err := s.updateCA(s.closeCtx); err != nil {
		return trace.Wrap(err)
	}

	s.mu.Lock()
	s.ldapInitialized = true
	s.scheduleNextLDAPCertRenewalLocked(windowsDesktopServiceCertTTL / 3)
	s.mu.Unlock()

	return nil
}

// scheduleNextLDAPCertRenewalLocked schedules a renewal of our LDAP credentials
// after some amount of time has elapsed. If an existing renewal is already
// scheduled, it is canceled and this new one takes its place.
//
// The lock on s.mu MUST be held.
func (s *WindowsService) scheduleNextLDAPCertRenewalLocked(after time.Duration) {
	if s.ldapCertRenew != nil {
		s.ldapCertRenew.Reset(after)
	} else {
		s.ldapCertRenew = time.AfterFunc(after, func() {
			if err := s.initializeLDAP(); err != nil {
				s.cfg.Log.WithError(err).Error("couldn't renew certificate for LDAP auth")
			}
		})
	}
}

func (s *WindowsService) startServiceHeartbeat() error {
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeCtx,
		Component:       teleport.ComponentWindowsDesktop,
		Mode:            srv.HeartbeatModeWindowsDesktopService,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   s.getServiceHeartbeatInfo,
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     defaults.HeartbeatCheckPeriod,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
		OnHeartbeat:     s.cfg.Heartbeat.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := heartbeat.Run(); err != nil {
			s.cfg.Log.WithError(err).Error("Heartbeat ended with error")
		}
	}()
	return nil
}

// startStaticHostHeartbeats spawns heartbeat routines for all static hosts in
// this service. We use heartbeats instead of registering once at startup to
// support expiration.
//
// When a WindowsService with a list of static hosts disappears, those hosts
// should eventually get cleaned up. But they should exist as long as the
// service itself is running.
func (s *WindowsService) startStaticHostHeartbeats() error {
	for _, host := range s.cfg.Heartbeat.StaticHosts {
		heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
			Context:         s.closeCtx,
			Component:       teleport.ComponentWindowsDesktop,
			Mode:            srv.HeartbeatModeWindowsDesktop,
			Announcer:       s.cfg.AccessPoint,
			GetServerInfo:   s.staticHostHeartbeatInfo(host, s.cfg.HostLabelsFn),
			KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
			AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
			CheckPeriod:     defaults.HeartbeatCheckPeriod,
			ServerTTL:       apidefaults.ServerAnnounceTTL,
			OnHeartbeat:     s.cfg.Heartbeat.OnHeartbeat,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		go func() {
			if err := heartbeat.Run(); err != nil {
				s.cfg.Log.WithError(err).Error("Heartbeat ended with error")
			}
		}()
	}
	return nil
}

// Close instructs the server to stop accepting new connections and abort all
// established ones. Close does not wait for the connections to be finished.
func (s *WindowsService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ldapCertRenew != nil {
		s.ldapCertRenew.Stop()
	}
	s.close()
	s.lc.close()
	return nil
}

// Serve starts serving TLS connections for plainLis. plainLis should be a TCP
// listener and Serve will handle TLS internally.
func (s *WindowsService) Serve(plainLis net.Listener) error {
	lis := tls.NewListener(plainLis, s.cfg.TLS)
	defer lis.Close()
	for {
		select {
		case <-s.closeCtx.Done():
			return trace.Wrap(s.closeCtx.Err())
		default:
		}
		conn, err := lis.Accept()
		if err != nil {
			if utils.IsOKNetworkError(err) || trace.IsConnectionProblem(err) {
				return nil
			}
			return trace.Wrap(err)
		}
		proxyConn, ok := conn.(*tls.Conn)
		if !ok {
			return trace.ConnectionProblem(nil, "Got %T from TLS listener, expected *tls.Conn", conn)
		}

		go s.handleConnection(proxyConn)
	}
}

func (s *WindowsService) ldapReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ldapInitialized
}

// handleConnection handles TLS connections from a Teleport proxy.
// It authenticates and authorizes the connection, and then begins
// translating the TDP messages from the proxy into native RDP.
func (s *WindowsService) handleConnection(proxyConn *tls.Conn) {
	log := s.cfg.Log

	tdpConn := tdp.NewConn(proxyConn)
	defer tdpConn.Close()

	// Inline function to enforce that we are centralizing TDP Error sending in this function.
	sendTDPError := func(message string) {
		if err := tdpConn.SendError(message); err != nil {
			log.Errorf("Failed to send TDP error message %v", err)
		}
	}

	// don't handle connections until the LDAP initialization retry loop has succeeded
	// (it would fail anyway, but this presents a better error to the user)
	if !s.ldapReady() {
		const msg = "This service cannot accept connections until LDAP initialization has completed."
		log.Error(msg)
		sendTDPError(msg)
		return
	}

	// Check connection limits.
	remoteAddr, _, err := net.SplitHostPort(proxyConn.RemoteAddr().String())
	if err != nil {
		log.WithError(err).Errorf("Could not parse client IP from %q", proxyConn.RemoteAddr().String())
		sendTDPError("Internal error.")
		return
	}
	log = log.WithField("client-ip", remoteAddr)
	if err := s.cfg.ConnLimiter.AcquireConnection(remoteAddr); err != nil {
		log.WithError(err).Warning("Connection limit exceeded, rejecting connection")
		sendTDPError("Connection limit exceeded.")
		return
	}
	defer s.cfg.ConnLimiter.ReleaseConnection(remoteAddr)

	// Authenticate the client.
	ctx, err := s.middleware.WrapContextWithUser(s.closeCtx, proxyConn)
	if err != nil {
		log.WithError(err).Warning("mTLS authentication failed for incoming connection")
		sendTDPError("Connection authentication failed.")
		return
	}
	log.Debug("Authenticated Windows desktop connection")

	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		log.WithError(err).Warning("authorization failed for Windows desktop connection")
		sendTDPError("Connection authorization failed.")
		return
	}

	// Fetch the target desktop info. Name of the desktop is passed via SNI.
	desktopName := strings.TrimSuffix(proxyConn.ConnectionState().ServerName, SNISuffix)
	log = log.WithField("desktop-name", desktopName)

	desktops, err := s.cfg.AccessPoint.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{HostID: s.cfg.Heartbeat.HostUUID, Name: desktopName})
	if err != nil {
		log.WithError(err).Warning("Failed to fetch desktop by name")
		sendTDPError("Teleport failed to find the requested desktop in its database.")
		return
	}
	if len(desktops) == 0 {
		log.Error("no windows desktops with HostID %s and Name %s", s.cfg.Heartbeat.HostUUID,
			desktopName)
		sendTDPError(fmt.Sprintf("Could not find desktop %v.", desktopName))
		return
	}
	desktop := desktops[0]

	log = log.WithField("desktop-addr", desktop.GetAddr())
	log.Debug("Connecting to Windows desktop")
	defer log.Debug("Windows desktop disconnected")

	if err := s.connectRDP(ctx, log, tdpConn, desktop, authContext); err != nil {
		log.Errorf("RDP connection failed: %v", err)
		sendTDPError("RDP connection failed.")
		return
	}
}

func (s *WindowsService) connectRDP(ctx context.Context, log logrus.FieldLogger, tdpConn *tdp.Conn, desktop types.WindowsDesktop, authCtx *auth.Context) error {
	identity := authCtx.Identity.GetIdentity()

	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	authPref, err := s.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	sessionID := session.NewID()

	// in order for the session to be recorded, the cluster's session recording mode must
	// not be "off" and the user's roles must enable recording
	recordSession := false
	if authCtx.Checker.RecordDesktopSession() {
		recConfig, err := s.cfg.AccessPoint.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		recordSession = recConfig.GetMode() != types.RecordOff
	} else {
		log.Infof("desktop session %v will not be recorded, user %v's roles disable recording", string(sessionID), authCtx.User.GetName())
	}

	var windowsUser string
	authorize := func(login string) error {
		windowsUser = login // capture attempted login user
		mfaParams := services.AccessMFAParams{
			Verified:       identity.MFAVerified != "",
			AlwaysRequired: authPref.GetRequireSessionMFA(),
		}
		return authCtx.Checker.CheckAccess(
			desktop,
			mfaParams,
			services.NewWindowsLoginMatcher(login))
	}

	// Use a context that is canceled when we're done handling
	// this connection. This ensures that the connection monitor
	// will stop checking for idle activity when the connection
	// is closed.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a session tracker so that other services, such as
	// the session upload completer, can track the session's lifetime.
	if err := s.trackSession(ctx, &identity, windowsUser, string(sessionID), desktop); err != nil {
		return trace.Wrap(err)
	}

	sw, err := s.newStreamWriter(recordSession, string(sessionID))
	if err != nil {
		return trace.Wrap(err)
	}

	// Closing the stream writer is needed to flush all recorded data
	// and trigger the upload. Do it in a goroutine since depending on
	// the session size it can take a while, and we don't want to block
	// the client.
	defer func() {
		go func() {
			if err := sw.Close(context.Background()); err != nil {
				log.WithError(err).Errorf("closing stream writer for desktop session %v", sessionID.String())
			}
		}()
	}()

	delay := timer()
	tdpConn.OnSend = s.makeTDPSendHandler(ctx, sw, delay, &identity, string(sessionID), desktop.GetAddr())
	tdpConn.OnRecv = s.makeTDPReceiveHandler(ctx, sw, delay, &identity, string(sessionID), desktop.GetAddr())

	sessionStartTime := s.cfg.Clock.Now().UTC().Round(time.Millisecond)
	rdpc, err := rdpclient.New(rdpclient.Config{
		Log: log,
		GenerateUserCert: func(ctx context.Context, username string, ttl time.Duration) (certDER, keyDER []byte, err error) {
			return s.generateCredentials(ctx, username, desktop.GetDomain(), ttl)
		},
		CertTTL:        windowsDesktopCertTTL,
		Addr:           desktop.GetAddr(),
		Conn:           tdpConn,
		AuthorizeFn:    authorize,
		AllowClipboard: authCtx.Checker.DesktopClipboard(),
		// AllowDirectorySharing() ensures this setting is modulated by build flag while in development
		AllowDirectorySharing: authCtx.Checker.DesktopDirectorySharing() && AllowDirectorySharing(),
	})
	if err != nil {
		s.onSessionStart(ctx, sw, &identity, sessionStartTime, windowsUser, string(sessionID), desktop, err)
		return trace.Wrap(err)
	}

	monitorCfg := srv.MonitorConfig{
		Context:           ctx,
		Conn:              tdpConn,
		Clock:             s.cfg.Clock,
		ClientIdleTimeout: authCtx.Checker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		Entry:             log,
		Emitter:           s.cfg.Emitter,
		LockWatcher:       s.cfg.LockWatcher,
		LockingMode:       authCtx.Checker.LockingMode(authPref.GetLockingMode()),
		LockTargets:       append(services.LockTargetsFromTLSIdentity(identity), types.LockTarget{WindowsDesktop: desktop.GetName()}),
		Tracker:           rdpc,
		TeleportUser:      identity.Username,
		ServerID:          s.cfg.Heartbeat.HostUUID,
		MessageWriter: &monitorErrorSender{
			log:     log,
			tdpConn: tdpConn,
		},
	}
	shouldDisconnectExpiredCert := authCtx.Checker.AdjustDisconnectExpiredCert(authPref.GetDisconnectExpiredCert())
	if shouldDisconnectExpiredCert && !identity.Expires.IsZero() {
		monitorCfg.DisconnectExpiredCert = identity.Expires
	}

	// UpdateClientActivity before starting monitor to
	// be doubly sure that the client isn't disconnected
	// due to an idle timeout before its had the chance to
	// call StartAndWait()
	rdpc.UpdateClientActivity()
	if err := srv.StartMonitor(monitorCfg); err != nil {
		// if we can't establish a connection monitor then we can't enforce RBAC.
		// consider this a connection failure and return an error
		// (in the happy path, rdpc remains open until Wait() completes)
		s.onSessionStart(ctx, sw, &identity, sessionStartTime, windowsUser, string(sessionID), desktop, err)
		return trace.Wrap(err)
	}

	s.onSessionStart(ctx, sw, &identity, sessionStartTime, windowsUser, string(sessionID), desktop, nil)
	err = rdpc.Run(ctx)
	s.onSessionEnd(ctx, sw, &identity, sessionStartTime, recordSession, windowsUser, string(sessionID), desktop)

	return trace.Wrap(err)
}

func (s *WindowsService) makeTDPSendHandler(ctx context.Context, emitter events.Emitter, delay func() int64,
	id *tlsca.Identity, sessionID, desktopAddr string) func(m tdp.Message, b []byte) {
	return func(m tdp.Message, b []byte) {
		switch b[0] {
		case byte(tdp.TypePNG2Frame), byte(tdp.TypePNGFrame), byte(tdp.TypeError):
			e := &events.DesktopRecording{
				Metadata: events.Metadata{
					Type: libevents.DesktopRecordingEvent,
					Time: s.cfg.Clock.Now().UTC().Round(time.Millisecond),
				},
				Message:           b,
				DelayMilliseconds: delay(),
			}
			if e.Size() > libevents.MaxProtoMessageSizeBytes {
				// Technically a PNG frame is unbounded and could be too big for a single protobuf.
				// In practice though, Windows limits RDP bitmaps to 64x64 pixels, and we compress
				// the PNGs before they get here, so most PNG frames are under 500 bytes. The largest
				// ones are around 2000 bytes. Anything approaching the limit of a single protobuf
				// is likely some sort of DoS attempt and not legitimate RDP traffic, so we don't log it.
				s.cfg.Log.Warnf("refusing to record %d byte PNG frame, image too large", len(b))
			} else if err := emitter.EmitAuditEvent(ctx, e); err != nil {
				s.cfg.Log.WithError(err).Warning("could not emit desktop recording event")
			}
		case byte(tdp.TypeClipboardData):
			if clip, ok := m.(tdp.ClipboardData); ok {
				// the TDP send handler emits a clipboard receive event, because we
				// received clipboard data from the remote desktop and are sending
				// it on the TDP connection
				s.onClipboardReceive(ctx, emitter, id, sessionID, desktopAddr, int32(len(clip)))
			}
		}
	}
}

func (s *WindowsService) makeTDPReceiveHandler(ctx context.Context, emitter events.Emitter, delay func() int64,
	id *tlsca.Identity, sessionID, desktopAddr string) func(m tdp.Message) {
	return func(m tdp.Message) {
		switch msg := m.(type) {
		case tdp.ClientScreenSpec, tdp.MouseButton, tdp.MouseMove:
			b, err := m.Encode()
			if err != nil {
				s.cfg.Log.WithError(err).Warning("could not emit desktop recording event")
			}
			e := &events.DesktopRecording{
				Metadata: events.Metadata{
					Type: libevents.DesktopRecordingEvent,
					Time: s.cfg.Clock.Now().UTC().Round(time.Millisecond),
				},
				Message:           b,
				DelayMilliseconds: delay(),
			}
			if e.Size() > libevents.MaxProtoMessageSizeBytes {
				// screen spec, mouse button, and mouse move are fixed size messages,
				// so they cannot exceed the maximum size
				s.cfg.Log.Warnf("refusing to record %d byte %T message", len(b), m)
			} else if err := emitter.EmitAuditEvent(ctx, e); err != nil {
				s.cfg.Log.WithError(err).Warning("could not emit desktop recording event")
			}
		case tdp.ClipboardData:
			// the TDP receive handler emits a clipboard send event, because we
			// received clipboard data from the user (over TDP) and are sending
			// it to the remote desktop
			s.onClipboardSend(ctx, emitter, id, sessionID, desktopAddr, int32(len(msg)))
		}
	}
}

func (s *WindowsService) getServiceHeartbeatInfo() (types.Resource, error) {
	srv, err := types.NewWindowsDesktopServiceV3(
		s.cfg.Heartbeat.HostUUID,
		types.WindowsDesktopServiceSpecV3{
			Addr:            s.cfg.Heartbeat.PublicAddr,
			TeleportVersion: teleport.Version,
			Hostname:        s.cfg.Hostname,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	return srv, nil
}

// staticHostHeartbeatInfo generates the Windows Desktop resource
// for heartbeating statically defined hosts
func (s *WindowsService) staticHostHeartbeatInfo(netAddr utils.NetAddr,
	getHostLabels func(string) map[string]string) func() (types.Resource, error) {
	return func() (types.Resource, error) {
		addr := netAddr.String()
		name, err := s.nameForStaticHost(addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// for static hosts, we match against the host's addr,
		// as the name is a randomly generated UUID
		labels := getHostLabels(addr)
		labels[types.OriginLabel] = types.OriginConfigFile
		desktop, err := types.NewWindowsDesktopV3(
			name,
			labels,
			types.WindowsDesktopSpecV3{
				Addr:   addr,
				Domain: s.cfg.Domain,
				HostID: s.cfg.Heartbeat.HostUUID,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		desktop.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
		return desktop, nil
	}
}

// nameForStaticHost attempts to find the name of an existing Windows desktop
// with the same address. If no matching address is found, a new name is
// generated.
//
// The list of WindowsDesktop objects should be read from the local cache. It
// should be reasonably fast to do this scan on every heartbeat. However, with
// a very large number of desktops in the cluster, this may use up a lot of CPU
// time.
func (s *WindowsService) nameForStaticHost(addr string) (string, error) {
	desktops, err := s.cfg.AccessPoint.GetWindowsDesktops(s.closeCtx,
		types.WindowsDesktopFilter{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, d := range desktops {
		if d.GetAddr() == addr {
			return d.GetName(), nil
		}
	}

	host, _, err := utils.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	parts := strings.Split(s.cfg.Heartbeat.HostUUID, "-")
	prefix := parts[len(parts)-1]
	return prefix + "-static-" + strings.ReplaceAll(host, ".", "-"), nil
}

// timer returns a closure that on each call returns the
// number of milliseconds that have elapsed since the first call.
// it returns 0 on the very first call.
func timer() func() int64 {
	var first time.Time
	return func() int64 {
		if first.IsZero() {
			first = time.Now()
			return 0
		}
		return int64(time.Since(first) / time.Millisecond)
	}
}

func (s *WindowsService) updateCA(ctx context.Context) error {
	// Publish the CA cert for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish their CAs so we don't
	// have to do it here.
	//
	// TODO(zmb3): support multiple CA certs per cluster (such as with HSMs).
	ca, err := s.cfg.AccessPoint.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.UserCA,
		DomainName: s.clusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err, "fetching Teleport CA: %v", err)
	}
	// LDAP stores certs and CRLs in binary DER format, so remove the outer PEM
	// wrapper.
	caPEM := ca.GetTrustedTLSKeyPairs()[0].Cert
	caBlock, _ := pem.Decode(caPEM)
	if caBlock == nil {
		return trace.BadParameter("failed to decode CA PEM block")
	}
	caDER := caBlock.Bytes

	crlDER, err := s.cfg.AccessPoint.GenerateCertAuthorityCRL(ctx, types.UserCA)
	if err != nil {
		return trace.Wrap(err, "generating CRL: %v", err)
	}

	// To make the CA trusted, we need 3 things:
	// 1. put the CA cert into the Trusted Certification Authorities in the
	//    Group Policy (done manually for now, see public docs)
	// 2. put the CA cert into NTAuth store in LDAP
	// 3. put the CRL of the CA into a dedicated LDAP entry
	//
	// Below we do #2 and #3.
	if err := s.updateCAInNTAuthStore(ctx, caDER); err != nil {
		return trace.Wrap(err, "updating NTAuth store over LDAP: %v", err)
	}
	if err := s.updateCRL(ctx, crlDER); err != nil {
		return trace.Wrap(err, "updating CRL over LDAP: %v", err)
	}
	return nil
}

// updateCAInNTAuthStore records the Teleport user CA in the Windows store which records
// CAs that are eligible to issue smart card login certificates and perform client
// private key archival.
//
// This function is equivalent to running:
//     certutil –dspublish –f <PathToCertFile.cer> NTAuthCA
//
// You can confirm the cert is present by running:
//     certutil -viewstore "ldap:///CN=NTAuthCertificates,CN=Public Key Services,CN=Services,CN=Configuration,DC=example,DC=com>?caCertificate"
//
// Once the CA is published to LDAP, it should eventually sync and be present in the
// machine's enterprise NTAuth store. You can check that with:
//     certutil -viewstore -enterprise NTAuth
//
// You can expedite the synchronization by running:
//     certutil -pulse
//
func (s *WindowsService) updateCAInNTAuthStore(ctx context.Context, caDER []byte) error {
	// Check if our CA is already in the store. The LDAP entry for NTAuth store
	// is constant and it should always exist.
	ntAuthDN := "CN=NTAuthCertificates,CN=Public Key Services,CN=Services,CN=Configuration," + s.cfg.LDAPConfig.domainDN()
	entries, err := s.lc.read(ntAuthDN, "certificationAuthority", []string{"cACertificate"})
	if err != nil {
		return trace.Wrap(err, "fetching existing CAs: %v", err)
	}
	if len(entries) != 1 {
		return trace.BadParameter("expected exactly 1 NTAuthCertificates CA store at %q, but found %d", ntAuthDN, len(entries))
	}
	// TODO(zmb3): during CA rotation, find the old CA in NTAuthStore and remove it.
	// Right now we just append the active CA and let the old ones hang around.
	existingCAs := entries[0].GetRawAttributeValues("cACertificate")
	for _, existingCADER := range existingCAs {
		// CA already present.
		if bytes.Equal(existingCADER, caDER) {
			s.cfg.Log.Info("Teleport CA already present in NTAuthStore in LDAP")
			return nil
		}
	}

	s.cfg.Log.Debugf("None of the %d existing NTAuthCertificates matched Teleport's", len(existingCAs))

	// CA is not in the store, append it.
	updatedCAs := make([]string, 0, len(existingCAs)+1)
	for _, existingCADER := range existingCAs {
		updatedCAs = append(updatedCAs, string(existingCADER))
	}
	updatedCAs = append(updatedCAs, string(caDER))

	if err := s.lc.update(ntAuthDN, map[string][]string{
		"cACertificate": updatedCAs,
	}); err != nil {
		return trace.Wrap(err, "updating CA entry: %v", err)
	}
	s.cfg.Log.Info("Added Teleport CA to NTAuthStore via LDAP")
	return nil
}

func (s *WindowsService) updateCRL(ctx context.Context, crlDER []byte) error {
	// Publish the CRL for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish CRLs of their CAs so we
	// don't have to do it here.
	//
	// CRLs live under the CDP (CRL Distribution Point) LDAP container. There's
	// another nested container with the CA name, I think, and then multiple
	// separate CRL objects in that container.
	//
	// We name our parent container "Teleport" and the CRL object is named
	// after the Teleport cluster name. For example, CRL for cluster "prod"
	// will be placed at:
	// ... > CDP > Teleport > prod
	containerDN := s.crlContainerDN()
	crlDN := s.crlDN()

	// Create the parent container.
	if err := s.lc.createContainer(containerDN); err != nil {
		return trace.Wrap(err, "creating CRL container: %v", err)
	}

	// Create the CRL object itself.
	if err := s.lc.create(
		crlDN,
		"cRLDistributionPoint",
		map[string][]string{"certificateRevocationList": {string(crlDER)}},
	); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// CRL already exists, update it.
		if err := s.lc.update(
			crlDN,
			map[string][]string{"certificateRevocationList": {string(crlDER)}},
		); err != nil {
			return trace.Wrap(err)
		}
		s.cfg.Log.Info("Updated CRL for Windows logins via LDAP")
	} else {
		s.cfg.Log.Info("Added CRL for Windows logins via LDAP")
	}
	return nil
}

// crlDN generates the LDAP distinguished name (DN) where this Windows Service
// will publish its certificate revocation list
func (s *WindowsService) crlDN() string {
	return "CN=" + s.clusterName + "," + s.crlContainerDN()
}

// crlContainerDN generates the LDAP distinguished name (DN) of the container
// where the certificate revocation list is published
func (s *WindowsService) crlContainerDN() string {
	return "CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration," + s.cfg.LDAPConfig.domainDN()
}

// generateCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
func (s *WindowsService) generateCredentials(ctx context.Context, username, domain string, ttl time.Duration) (certDER, keyDER []byte, err error) {
	// Important: rdpclient currently only supports 2048-bit RSA keys.
	// If you switch the key type here, update handle_general_authentication in
	// rdp/rdpclient/src/piv.rs accordingly.
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Also important: rdpclient expects the private key to be in PKCS1 format.
	keyDER = x509.MarshalPKCS1PrivateKey(rsaKey)

	// Generate the Windows-compatible certificate, see
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	// for requirements.
	san, err := subjectAltNameExtension(username, domain)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	csr := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: username},
		// We have to pass SAN and ExtKeyUsage as raw extensions because
		// crypto/x509 doesn't support what we need:
		// - x509.ExtKeyUsage doesn't have the Smartcard Logon variant
		// - x509.CertificateRequest doesn't have OtherName SAN fields (which
		//   is a type of SAN distinct from DNSNames, EmailAddresses, IPAddresses
		//   and URIs)
		ExtraExtensions: []pkix.Extension{
			enhancedKeyUsageExtension,
			san,
		},
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, rsaKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	// Note: this CRL DN may or may not be the same DN published in updateCRL.
	//
	// There can be multiple AD domains connected to Teleport. Each
	// windows_desktop_service is connected to a single AD domain and publishes
	// CRLs in it. Each service can also handle RDP connections for a different
	// domain, with the assumption that some other windows_desktop_service
	// published a CRL there.
	crlDN := s.crlDN()
	genResp, err := s.cfg.AuthClient.GenerateWindowsDesktopCert(ctx, &proto.WindowsDesktopCertRequest{
		CSR: csrPEM,
		// LDAP URI pointing at the CRL created with updateCRL.
		//
		// The full format is:
		// ldap://domain_controller_addr/distinguished_name_and_parameters.
		//
		// Using ldap:///distinguished_name_and_parameters (with empty
		// domain_controller_addr) will cause Windows to fetch the CRL from any
		// of its current domain controllers.
		CRLEndpoint: fmt.Sprintf("ldap:///%s?certificateRevocationList?base?objectClass=cRLDistributionPoint", crlDN),
		TTL:         proto.Duration(ttl),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certBlock, _ := pem.Decode(genResp.Cert)
	certDER = certBlock.Bytes
	return certDER, keyDER, nil
}

// trackSession creates a session tracker for the given sessionID and
// attributes, and starts a goroutine to continually extend the tracker
// expiration while the session is active. Once the given ctx is closed,
// the tracker will be marked as terminated.
func (s *WindowsService) trackSession(ctx context.Context, id *tlsca.Identity, windowsUser string, sessionID string, desktop types.WindowsDesktop) error {
	trackerSpec := types.SessionTrackerSpecV1{
		SessionID:   sessionID,
		Kind:        string(types.WindowsDesktopSessionKind),
		State:       types.SessionState_SessionStateRunning,
		Hostname:    s.cfg.Hostname,
		Address:     desktop.GetAddr(),
		DesktopName: desktop.GetName(),
		ClusterName: s.clusterName,
		Login:       windowsUser,
		Participants: []types.Participant{{
			User: id.Username,
		}},
		HostUser: id.Username,
		Created:  s.cfg.Clock.Now(),
	}

	s.cfg.Log.Debugf("Creating tracker for session %v", sessionID)
	tracker, err := srv.NewSessionTracker(ctx, trackerSpec, s.cfg.AuthClient)
	if err != nil {
		// Ignore access denied errors, which we will get if the auth
		// server is v9.2.1 or earlier, since only node, proxy, and
		// kube roles had permission to create session trackers.
		if trace.IsAccessDenied(err) {
			s.cfg.Log.Debugf("Insufficient permissions to create session tracker, skipping session tracking for session %v", sessionID)
			return nil
		}
		return trace.Wrap(err)
	}

	go func() {
		if err := tracker.UpdateExpirationLoop(ctx, s.cfg.Clock); err != nil {
			s.cfg.Log.WithError(err).Warnf("Failed to update session tracker expiration for session %v", sessionID)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := tracker.Close(s.closeCtx); err != nil {
			s.cfg.Log.WithError(err).Debugf("Failed to close session tracker for session %v", sessionID)
		}
	}()

	return nil
}

// The following vars contain the various object identifiers required for smartcard
// login certificates.
//
// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
var (
	// enhancedKeyUsageExtensionOID is the object identifier for a
	// certificate's enhanced key usage extension
	enhancedKeyUsageExtensionOID = asn1.ObjectIdentifier{2, 5, 29, 37}

	// subjectAltNameExtensionOID is the object identifier for a
	// certificate's subject alternative name extension
	subjectAltNameExtensionOID = asn1.ObjectIdentifier{2, 5, 29, 17}

	// clientAuthenticationOID is the object idnetifier that is used to
	// include client SSL authentication in a certificate's enhanced
	// key usage
	clientAuthenticationOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}

	// smartcardLogonOID is the object identifier that is used to include
	// smartcard login in a certificate's enhanced key usage
	smartcardLogonOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 2}

	// upnOtherNameOID is the object identifier that is used to include
	// the user principal name in a certificate's subject alternative name
	upnOtherNameOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 3}
)

var enhancedKeyUsageExtension = pkix.Extension{
	Id: enhancedKeyUsageExtensionOID,
	Value: func() []byte {
		val, err := asn1.Marshal([]asn1.ObjectIdentifier{
			clientAuthenticationOID,
			smartcardLogonOID,
		})
		if err != nil {
			panic(err)
		}
		return val
	}(),
}

func subjectAltNameExtension(user, domain string) (pkix.Extension, error) {
	// Setting otherName SAN according to
	// https://samfira.com/2020/05/16/golang-x-509-certificates-and-othername/
	//
	// othernName SAN is needed to pass the UPN of the user, per
	// https://docs.microsoft.com/en-us/troubleshoot/windows-server/windows-security/enabling-smart-card-logon-third-party-certification-authorities
	ext := pkix.Extension{Id: subjectAltNameExtensionOID}
	var err error
	ext.Value, err = asn1.Marshal(
		subjectAltName{
			OtherName: otherName{
				OID: upnOtherNameOID,
				Value: upn{
					Value: fmt.Sprintf("%s@%s", user, domain), // TODO(zmb3): sanitize username to avoid domain spoofing
				},
			},
		},
	)
	if err != nil {
		return ext, trace.Wrap(err)
	}
	return ext, nil
}

// Types for ASN.1 SAN serialization.

type subjectAltName struct {
	OtherName otherName `asn1:"tag:0"`
}

type otherName struct {
	OID   asn1.ObjectIdentifier
	Value upn `asn1:"tag:0"`
}

type upn struct {
	Value string `asn1:"utf8"`
}

// monitorErrorSender implements the io.StringWriter
// interface in order to allow us to pass connection
// monitor disconnect messages back to the frontend
// over the tdp.Conn
type monitorErrorSender struct {
	log     logrus.FieldLogger
	tdpConn *tdp.Conn
}

func (m *monitorErrorSender) WriteString(s string) (n int, err error) {
	if err := m.tdpConn.SendError(s); err != nil {
		errMsg := fmt.Sprintf("Failed to send TDP error message %v: %v", s, err)
		m.log.Error(errMsg)
		return 0, trace.Errorf(errMsg)
	}

	return len(s), nil
}
