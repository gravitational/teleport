/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package desktop

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/windows"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/defaults"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service/servicecfg"
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
	ldapDialTimeout = 15 * time.Second

	// ldapRequestTimeout is the timeout for making LDAP requests.
	// It is larger than the dial timeout because LDAP queries in large
	// Active Directory environments may take longer to complete.
	ldapRequestTimeout = 45 * time.Second

	// windowsDesktopServiceCertTTL is the TTL for certificates issued to the
	// Windows Desktop Service in order to authenticate with the LDAP server.
	// It is set longer than the Windows certificates for users because it is
	// not used for interactive login and is only used when issuing certs for
	// a restrictive service account.
	windowsDesktopServiceCertTTL = 8 * time.Hour

	// windowsDesktopServiceCertRetryInterval indicates how often to retry
	// issuing an LDAP certificate if the operation fails.
	windowsDesktopServiceCertRetryInterval = 10 * time.Minute

	// ldapTimeoutRetryInterval indicates how often to retry LDAP initialization
	// if it times out. It is set lower than windowsDesktopServiceCertRetryInterval
	// because LDAP timeouts may indicate a temporary issue.
	ldapTimeoutRetryInterval = 10 * time.Second
)

// ComputerAttributes are the attributes we fetch when discovering
// Windows hosts via LDAP
// see: https://docs.microsoft.com/en-us/windows/win32/adschema/c-computer#windows-server-2012-attributes
var ComputerAttributes = []string{
	windows.AttrName,
	windows.AttrCommonName,
	windows.AttrDistinguishedName,
	windows.AttrDNSHostName,
	windows.AttrObjectGUID,
	windows.AttrOS,
	windows.AttrOSVersion,
	windows.AttrPrimaryGroupID,
}

// WindowsService implements the RDP-based Windows desktop access service.
//
// This service accepts mTLS connections from the proxy, establishes RDP
// connections to Windows hosts and translates RDP into Teleport's desktop
// protocol.
type WindowsService struct {
	cfg        WindowsServiceConfig
	middleware *auth.Middleware

	ca *windows.CertificateStoreClient
	lc *windows.LDAPClient

	mu              sync.Mutex // mu protects the fields that follow
	ldapConfigured  bool
	ldapInitialized bool
	ldapCertRenew   *time.Timer

	// lastDisoveryResults stores the results of the most recent LDAP search
	// when desktop discovery is enabled.
	// no synchronization is necessary because this is only read/written from
	// the reconciler goroutine.
	lastDiscoveryResults map[string]types.WindowsDesktop

	// Windows hosts discovered via LDAP likely won't resolve with the
	// default DNS resolver, so we need a custom resolver that will
	// query the domain controller.
	dnsResolver *net.Resolver

	// clusterName is the cached local cluster name, to avoid calling
	// cfg.AccessPoint.GetClusterName multiple times.
	clusterName string

	// auditCache caches information from shared directory
	// TDP messages that are needed for
	// creating shared directory audit events.
	auditCache sharedDirectoryAuditCache

	// NLA indicates whether this service will attempt to perform
	// Network Level Authentication (NLA) when attempting to connect
	// to domain-joined Windows hosts
	enableNLA bool

	closeCtx context.Context
	close    func()
}

// WindowsServiceConfig contains all necessary configuration values for a
// WindowsService.
type WindowsServiceConfig struct {
	// Logger is the logger for the service.
	Logger *slog.Logger
	// Clock provides current time.
	Clock        clockwork.Clock
	DataDir      string
	LicenseStore rdpclient.LicenseStore
	// Authorizer is used to authorize requests.
	Authorizer authz.Authorizer
	// LockWatcher is used to monitor for new locks.
	LockWatcher *services.LockWatcher
	// Emitter emits audit log events.
	Emitter events.Emitter
	// TLS is the TLS server configuration.
	TLS *tls.Config
	// AccessPoint is the Auth API client (with caching).
	AccessPoint authclient.WindowsDesktopAccessPoint
	// AuthClient is the Auth API client (without caching).
	AuthClient authclient.ClientI
	// ConnLimiter limits the number of active connections per client IP.
	ConnLimiter *limiter.ConnectionsLimiter
	// Heartbeat contains configuration for service heartbeats.
	Heartbeat HeartbeatConfig
	// HostLabelsFn gets labels that should be applied to a Windows host.
	HostLabelsFn func(host string) map[string]string
	// ShowDesktopWallpaper determines whether desktop sessions will show a
	// user-selected wallpaper vs a system-default, single-color wallpaper.
	ShowDesktopWallpaper bool
	// LDAPConfig contains parameters for connecting to an LDAP server.
	// LDAP functionality is disabled if Addr is empty.
	windows.LDAPConfig
	// PKIDomain optionally configures a separate Active Directory domain
	// for PKI operations. If empty, the domain from the LDAP config is used.
	// This can be useful for cases where PKI is configured in a root domain
	// but Teleport is used to provide access to users and computers in a child
	// domain.
	PKIDomain string
	// KCDAddr optionally configures address of Key Distribution Center used during Kerberos NLA negotiation.
	// If empty LDAP address will be used.
	// Used for NLA support when AD is true.
	KDCAddr string
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
	// Hostname of the Windows desktop service
	Hostname string
	// ConnectedProxyGetter gets the proxies teleport is connected to.
	ConnectedProxyGetter *reversetunnel.ConnectedProxyGetter
	Labels               map[string]string
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
	// StaticHosts is an optional list of static Windows hosts to register
	StaticHosts []servicecfg.WindowsHost
}

func (cfg *WindowsServiceConfig) checkAndSetDiscoveryDefaults() error {
	switch {
	case cfg.DiscoveryBaseDN == types.Wildcard:
		cfg.DiscoveryBaseDN = cfg.DomainDN()
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
	if cfg.LDAPConfig.Addr != "" {
		if err := cfg.LDAPConfig.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := cfg.checkAndSetDiscoveryDefaults(); err != nil {
		return trace.Wrap(err)
	}

	cfg.Logger = cmp.Or(cfg.Logger, slog.With(teleport.ComponentKey, teleport.ComponentWindowsDesktop))
	cfg.Clock = cmp.Or(cfg.Clock, clockwork.NewRealClock())
	cfg.ConnectedProxyGetter = cmp.Or(cfg.ConnectedProxyGetter, reversetunnel.NewConnectedProxyGetter())

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

const insecureSkipVerifyWarning = "LDAP configuration specifies both a CA certificate and insecure_skip_verify. " +
	"TLS connections to the LDAP server will not be verified. If this is intentional, disregard this warning."

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
		cfg.Logger.WarnContext(context.Background(), insecureSkipVerifyWarning)
	}

	clusterName, err := cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "fetching cluster name")
	}

	var resolver *net.Resolver
	if cfg.LDAPConfig.Addr != "" {
		// Here we assume the LDAP server is an Active Directory Domain Controller,
		// which means it should also be a DNS server that can resolve Windows hosts.
		dnsServer, _, err := net.SplitHostPort(cfg.LDAPConfig.Addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dnsAddr := net.JoinHostPort(dnsServer, "53")
		cfg.Logger.DebugContext(context.Background(), "DNS lookups will be performed against", "addr", dnsAddr)
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// Ignore the address provided, and always explicitly dial
				// the domain controller.
				d := net.Dialer{Timeout: dnsDialTimeout}
				return d.DialContext(ctx, network, dnsAddr)
			},
		}
	}

	clustername, err := cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, close := context.WithCancel(context.Background())
	s := &WindowsService{
		cfg: cfg,
		middleware: &auth.Middleware{
			ClusterName:   clustername.GetClusterName(),
			AcceptedUsage: []string{teleport.UsageWindowsDesktopOnly},
		},
		dnsResolver: resolver,
		lc:          &windows.LDAPClient{Cfg: cfg.LDAPConfig},
		clusterName: clusterName.GetClusterName(),
		closeCtx:    ctx,
		close:       close,
		auditCache:  newSharedDirectoryAuditCache(),

		// For now, NLA is opt-in via an environment variable.
		// We'll make it the default behavior in a future release.
		enableNLA: os.Getenv("TELEPORT_ENABLE_RDP_NLA") == "yes",
	}

	caLDAPConfig := s.cfg.LDAPConfig
	if s.cfg.PKIDomain != "" {
		caLDAPConfig.Domain = s.cfg.PKIDomain
	}
	s.cfg.Logger.InfoContext(ctx, "PKI domain configured", "domain", caLDAPConfig.Domain)

	s.ca = windows.NewCertificateStoreClient(windows.CertificateStoreConfig{
		AccessPoint: s.cfg.AccessPoint,
		LDAPConfig:  caLDAPConfig,
		Log:         logrus.NewEntry(logrus.StandardLogger()),
		ClusterName: s.clusterName,
		LC:          s.lc,
	})

	if caLDAPConfig.Addr != "" {
		s.ldapConfigured = true
		// initialize LDAP - if this fails it will automatically schedule a retry.
		// we don't want to return an error in this case, because failure to start
		// the service brings down the entire Teleport process
		if err := s.initializeLDAP(); err != nil {
			s.cfg.Logger.ErrorContext(ctx, "initializing LDAP client, will retry", "error", err)
		}
	}

	ok := false
	defer func() {
		if !ok {
			s.Close()
		}
	}()

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
		s.cfg.Logger.WarnContext(ctx, "desktop discovery via LDAP is disabled, and no hosts are defined in the configuration; there will be no Windows desktops available to connect")
	} else {
		s.cfg.Logger.InfoContext(ctx, "desktop discovery via LDAP is disabled, set 'base_dn' to enable")
	}

	// if LDAP-based discovery is not enabled, but we have configured LDAP
	// then it's important that we periodically try to use the LDAP connection
	// to detect connection closure
	if s.ldapConfigured && len(s.cfg.DiscoveryBaseDN) == 0 {
		s.startLDAPConnectionCheck(ctx)
	}

	ok = true
	return s, nil
}

// startLDAPConnectionCheck starts a background process that
// periodically reads from the LDAP connection in order to detect
// connection closure, and reconnects if necessary.
// This is useful when LDAP-based discovery is disabled, because without
// discovery the connection goes idle and may be closed by the server.
func (s *WindowsService) startLDAPConnectionCheck(ctx context.Context) {
	s.cfg.Logger.DebugContext(ctx, "starting LDAP connection checker")
	go func() {
		t := s.cfg.Clock.NewTicker(5 * time.Minute)
		defer t.Stop()

		for {
			select {
			case <-t.Chan():
				// First check if we have successfully initialized the LDAP client.
				// If not, then do that now and return.
				// (This mimics the check that is performed when LDAP discovery is enabled.)
				s.mu.Lock()
				if !s.ldapInitialized {
					s.cfg.Logger.DebugContext(context.Background(), "LDAP not ready, attempting to reconnect")
					s.mu.Unlock()
					s.initializeLDAP()
					return
				}
				s.mu.Unlock()

				// If we have initialized the LDAP client, then try to use it to make sure we're still connected
				// by attempting to read CAs in the NTAuth store (we know we have permissions to do so).
				ntAuthDN := "CN=NTAuthCertificates,CN=Public Key Services,CN=Services,CN=Configuration," + s.cfg.LDAPConfig.DomainDN()
				_, err := s.lc.Read(ntAuthDN, "certificationAuthority", []string{"cACertificate"})
				if trace.IsConnectionProblem(err) {
					s.cfg.Logger.DebugContext(ctx, "detected broken LDAP connection, will reconnect")
					if err := s.initializeLDAP(); err != nil {
						s.cfg.Logger.WarnContext(ctx, "failed to reconnect to LDAP", "error", err)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *WindowsService) newSessionRecorder(recConfig types.SessionRecordingConfig, sessionID string) (libevents.SessionPreparerRecorder, error) {
	return recorder.New(recorder.Config{
		SessionID:    session.ID(sessionID),
		ServerID:     s.cfg.Heartbeat.HostUUID,
		Namespace:    apidefaults.Namespace,
		Clock:        s.cfg.Clock,
		ClusterName:  s.clusterName,
		RecordingCfg: recConfig,
		SyncStreamer: s.cfg.AuthClient,
		DataDir:      s.cfg.DataDir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentWindowsDesktop),
		// Session stream is using server context, not session context,
		// to make sure that session is uploaded even after it is closed
		Context: s.closeCtx,
	})
}

func (s *WindowsService) tlsConfigForLDAP() (*tls.Config, error) {
	// trim NETBIOS name from username
	user := s.cfg.Username
	if i := strings.LastIndex(s.cfg.Username, `\`); i != -1 {
		user = user[i+1:]
	}
	if s.cfg.SID == "" {
		s.cfg.Logger.WarnContext(context.Background(), "LDAP configuration is missing service account SID")
	}
	certDER, keyDER, err := s.generateCredentials(s.closeCtx, generateCredentialsRequest{
		username:           user,
		domain:             s.cfg.Domain,
		ttl:                windowsDesktopServiceCertTTL,
		activeDirectorySID: s.cfg.SID,
		omitCDP:            true,
	})
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
		ServerName:         s.cfg.ServerName,
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
//
// This method is safe for concurrent calls.
func (s *WindowsService) initializeLDAP() error {
	tc, err := s.tlsConfigForLDAP()
	if trace.IsAccessDenied(err) && modules.GetModules().BuildType() == modules.BuildEnterprise {
		s.cfg.Logger.WarnContext(context.Background(),
			"Could not generate certificate for LDAPS. Ensure that the auth server is licensed for desktop access.")
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

	conn, err := ldap.DialURL(
		"ldaps://"+s.cfg.Addr,
		ldap.DialWithDialer(&net.Dialer{Timeout: ldapDialTimeout}),
		ldap.DialWithTLSConfig(tc),
	)
	if err != nil {
		s.mu.Lock()
		s.ldapInitialized = false

		// failures due to timeouts might be transient, so retry more frequently
		retryAfter := windowsDesktopServiceCertRetryInterval
		if errors.Is(err, context.DeadlineExceeded) {
			retryAfter = ldapTimeoutRetryInterval
		}

		s.scheduleNextLDAPCertRenewalLocked(retryAfter)
		s.mu.Unlock()
		return trace.Wrap(err, "dial")
	}

	conn.SetTimeout(ldapRequestTimeout)
	s.lc.SetClient(conn)

	if err := s.ca.Update(s.closeCtx); err != nil {
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
	s.cfg.Logger.InfoContext(context.Background(), "scheduled next LDAP cert renewal", "duration", after)
	if s.ldapCertRenew != nil {
		s.ldapCertRenew.Reset(after)
	} else {
		s.ldapCertRenew = time.AfterFunc(after, func() {
			if err := s.initializeLDAP(); err != nil {
				s.cfg.Logger.ErrorContext(context.Background(), "couldn't renew certificate for LDAP auth", "error", err)
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
			s.cfg.Logger.ErrorContext(s.closeCtx, "service heartbeat ended", "error", err)
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
		if err := s.startStaticHostHeartbeat(host); err != nil {
			return err
		}
	}
	return nil
}

// startStaticHostHeartbeats spawns heartbeat goroutine for single host
func (s *WindowsService) startStaticHostHeartbeat(host servicecfg.WindowsHost) error {
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeCtx,
		Component:       teleport.ComponentWindowsDesktop,
		Mode:            srv.HeartbeatModeWindowsDesktop,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   s.staticHostHeartbeatInfo(host, s.cfg.HostLabelsFn),
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL(),
		AnnouncePeriod:  apidefaults.ServerAnnounceTTL/2 + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
		CheckPeriod:     5 * time.Minute,
		ServerTTL:       apidefaults.ServerAnnounceTTL,
		OnHeartbeat:     s.cfg.Heartbeat.OnHeartbeat,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := heartbeat.Run(); err != nil {
			s.cfg.Logger.ErrorContext(s.closeCtx, "static host heartbeat ended", "error", err)
		}
	}()
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
	s.lc.Close()
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

func (s *WindowsService) readyForConnections() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// If LDAP was not configured, we assume all hosts are non-AD
	// and the server can accept connections right away.
	if !s.ldapConfigured {
		return true
	}

	// If LDAP was configured, then we need to wait for it to be initialized
	// before accepting connections.
	return s.ldapInitialized
}

// handleConnection handles TLS connections from a Teleport proxy.
// It authenticates and authorizes the connection, and then begins
// translating the TDP messages from the proxy into native RDP.
func (s *WindowsService) handleConnection(proxyConn *tls.Conn) {
	log := s.cfg.Logger

	tdpConn := tdp.NewConn(proxyConn)
	defer tdpConn.Close()

	// Inline function to enforce that we are centralizing TDP Error sending in this function.
	sendTDPError := func(message string) {
		if err := tdpConn.SendNotification(message, tdp.SeverityError); err != nil {
			log.ErrorContext(context.Background(), "Failed to send TDP error message", "error", err)
		}
	}

	// don't handle connections until the LDAP initialization retry loop has succeeded
	// (it would fail anyway, but this presents a better error to the user)
	if !s.readyForConnections() {
		const msg = "This service cannot accept connections until LDAP initialization has completed."
		log.ErrorContext(context.Background(), msg)
		sendTDPError(msg)
		return
	}

	// Check connection limits.
	remoteAddr, _, err := net.SplitHostPort(proxyConn.RemoteAddr().String())
	if err != nil {
		log.ErrorContext(context.Background(), "Could not parse client IP", "addr", proxyConn.RemoteAddr().String(), "error", err)
		sendTDPError("Internal error.")
		return
	}
	log = log.With("client_ip", remoteAddr)
	if err := s.cfg.ConnLimiter.AcquireConnection(remoteAddr); err != nil {
		log.WarnContext(context.Background(), "Connection limit exceeded, rejecting connection")
		sendTDPError("Connection limit exceeded.")
		return
	}
	defer s.cfg.ConnLimiter.ReleaseConnection(remoteAddr)

	// Authenticate the client.
	ctx, err := s.middleware.WrapContextWithUser(s.closeCtx, proxyConn)
	if err != nil {
		log.WarnContext(ctx, "mTLS authentication failed for incoming connection", "error", err)
		sendTDPError("Connection authentication failed.")
		return
	}
	log.DebugContext(ctx, "Authenticated Windows desktop connection")

	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		log.WarnContext(ctx, "authorization failed for Windows desktop connection", "error", err)
		sendTDPError("Connection authorization failed.")
		return
	}

	// Fetch the target desktop info. Name of the desktop is passed via SNI.
	desktopName := strings.TrimSuffix(proxyConn.ConnectionState().ServerName, SNISuffix)
	log = log.With("desktop_name", desktopName)

	desktops, err := s.cfg.AccessPoint.GetWindowsDesktops(ctx,
		types.WindowsDesktopFilter{HostID: s.cfg.Heartbeat.HostUUID, Name: desktopName})
	if err != nil {
		log.WarnContext(ctx, "Failed to fetch desktop by name", "error", err)
		sendTDPError("Teleport failed to find the requested desktop in its database.")
		return
	}
	if len(desktops) == 0 {
		log.ErrorContext(ctx, "desktop not found", "host_uuid", s.cfg.Heartbeat.HostUUID, "name", desktopName)
		sendTDPError(fmt.Sprintf("Could not find desktop %v.", desktopName))
		return
	}
	desktop := desktops[0]

	log = log.With("desktop_addr", desktop.GetAddr())
	log.DebugContext(ctx, "Connecting to Windows desktop")
	defer log.DebugContext(ctx, "Windows desktop disconnected")

	if err := s.connectRDP(ctx, log, tdpConn, desktop, authContext); err != nil {
		log.ErrorContext(context.Background(), "RDP connection failed", "error", err)
		msg := "RDP connection failed."
		var um trace.UserMessager
		if errors.As(err, &um) {
			msg = um.UserMessage()
		}
		sendTDPError(msg)
		return
	}
}

func (s *WindowsService) connectRDP(ctx context.Context, log *slog.Logger, tdpConn *tdp.Conn, desktop types.WindowsDesktop, authCtx *authz.Context) error {
	identity := authCtx.Identity.GetIdentity()

	log = log.With("teleport_user", identity.Username, "desktop_addr", desktop.GetAddr(), "ad", !desktop.NonAD())

	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	authPref, err := s.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	addr, err := utils.ParseHostPortAddr(desktop.GetAddr(), defaults.RDPListenPort)
	if err != nil {
		return trace.Wrap(err)
	}

	sessionID := session.NewID()
	log = log.With("session_id", sessionID)

	// in order for the session to be recorded, the cluster's session recording mode must
	// not be "off" and the user's roles must enable recording
	var recConfig types.SessionRecordingConfig
	var recordSession bool
	if !authCtx.Checker.RecordDesktopSession() {
		recConfig = types.DefaultSessionRecordingConfig()
		recConfig.SetMode(types.RecordOff)
		log.InfoContext(ctx, "desktop session will not be recorded, user's roles disable recording")
	} else {
		recConfig, err = s.cfg.AccessPoint.GetSessionRecordingConfig(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		recordSession = recConfig.GetMode() != types.RecordOff
	}

	// Use a context that is canceled when we're done handling
	// this connection. This ensures that the connection monitor
	// will stop checking for idle activity when the connection
	// is closed.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	authorize := func(login string) error {
		state := authCtx.GetAccessState(authPref)
		return authCtx.Checker.CheckAccess(
			desktop,
			state,
			services.NewWindowsLoginMatcher(login))
	}

	recorder, err := s.newSessionRecorder(recConfig, string(sessionID))
	if err != nil {
		return trace.Wrap(err)
	}

	// Closing the stream writer is needed to flush all recorded data
	// and trigger the upload. Do it in a goroutine since depending on
	// the session size it can take a while, and we don't want to block
	// the client.
	defer func() {
		go func() {
			if err := recorder.Close(context.Background()); err != nil {
				log.ErrorContext(context.Background(), "closing stream writer for desktop", "session_id", sessionID.String())
			}
		}()
	}()

	// We won't have the windows username until we start to read from the websocket,
	// but we need to start emitting audit events now. Create an auditor without
	// specifying the username (we'll update it soon as we have it).
	audit := s.newSessionAuditor(string(sessionID), &identity, "", desktop)

	groups, err := authCtx.Checker.DesktopGroups(desktop)
	if err != nil && !trace.IsAccessDenied(err) {
		startEvent := audit.makeSessionStart(err)
		s.record(ctx, recorder, startEvent)
		s.emit(ctx, startEvent)
		return trace.Wrap(err)
	}
	createUsers := err == nil

	// it's important that we set the OnSend and OnRecv handlers prior to
	// initializing the client so that we capture all relevant data in the
	// session recording
	delay := timer()
	tdpConn.OnSend = s.makeTDPSendHandler(ctx, recorder, delay, tdpConn, audit)
	tdpConn.OnRecv = s.makeTDPReceiveHandler(ctx, recorder, delay, tdpConn, audit)

	width, height := desktop.GetScreenSize()
	log = log.With("screen_size", fmt.Sprintf("%dx%d", width, height))

	computerName, ok := desktop.GetLabel(types.DiscoveryLabelWindowsComputerName)
	if !ok {
		if computerName, err = utils.Host(desktop.GetAddr()); err != nil {
			return trace.Wrap(err, "DNS host name is not specified and desktop address is invalid")
		}
		// sspi-rs returns misleading error when IP is used as a computer name,
		// so we replace it with host name that will still not match anything
		// in KDC registry but error returned will be more consistent with other
		// similar cases
		if len(net.ParseIP(computerName)) != 0 {
			computerName = "missing.computer.name"
		}
	}
	log = log.With("computer_name", computerName)

	kdcAddr := s.cfg.KDCAddr
	if !desktop.NonAD() && kdcAddr == "" && s.cfg.LDAPConfig.Addr != "" {
		if kdcAddr, err = utils.Host(s.cfg.LDAPConfig.Addr); err != nil {
			return trace.Wrap(err, "KDC address is unspecified and LDAP address is invalid")
		}
	}

	nla := s.enableNLA && !desktop.NonAD()

	log = log.With("kdc_addr", kdcAddr, "nla", nla)
	log.InfoContext(context.Background(), "initiating RDP client")

	//nolint:staticcheck // SA4023. False positive, depends on build tags.
	rdpc, err := rdpclient.New(rdpclient.Config{
		LicenseStore: s.cfg.LicenseStore,
		HostID:       s.cfg.Heartbeat.HostUUID,
		Logger:       log,
		GenerateUserCert: func(ctx context.Context, username string, ttl time.Duration) (certDER, keyDER []byte, err error) {
			return s.generateUserCert(ctx, username, ttl, desktop, createUsers, groups)
		},
		CertTTL:               windows.CertTTL,
		Addr:                  addr.String(),
		ComputerName:          computerName,
		KDCAddr:               kdcAddr,
		Conn:                  tdpConn,
		AuthorizeFn:           authorize,
		AllowClipboard:        authCtx.Checker.DesktopClipboard(),
		AllowDirectorySharing: authCtx.Checker.DesktopDirectorySharing(),
		ShowDesktopWallpaper:  s.cfg.ShowDesktopWallpaper,
		Width:                 width,
		Height:                height,
		AD:                    !desktop.NonAD(),
		NLA:                   nla,
	})
	// before we check the error above, we grab the Windows user so that
	// future audit events include the proper username
	var windowsUser string
	if rdpc != nil {
		windowsUser = rdpc.GetClientUsername()
		audit.windowsUser = windowsUser
	}
	//nolint:staticcheck // SA4023. False positive, depends on build tags.
	if err != nil {
		startEvent := audit.makeSessionStart(err)
		s.record(ctx, recorder, startEvent)
		s.emit(ctx, startEvent)
		return trace.Wrap(err)
	}

	if err := s.trackSession(ctx, &identity, windowsUser, string(sessionID), desktop); err != nil {
		return trace.Wrap(err)
	}

	monitorCfg := srv.MonitorConfig{
		Context:               ctx,
		Conn:                  tdpConn,
		Clock:                 s.cfg.Clock,
		ClientIdleTimeout:     authCtx.Checker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		DisconnectExpiredCert: authCtx.GetDisconnectCertExpiry(authPref),
		Entry:                 logrus.NewEntry(logrus.StandardLogger()),
		Emitter:               s.cfg.Emitter,
		EmitterContext:        s.closeCtx,
		LockWatcher:           s.cfg.LockWatcher,
		LockingMode:           authCtx.Checker.LockingMode(authPref.GetLockingMode()),
		LockTargets:           append(services.LockTargetsFromTLSIdentity(identity), types.LockTarget{WindowsDesktop: desktop.GetName()}),
		Tracker:               rdpc,
		TeleportUser:          identity.Username,
		ServerID:              s.cfg.Heartbeat.HostUUID,
		IdleTimeoutMessage:    netConfig.GetClientIdleTimeoutMessage(),
		MessageWriter:         &monitorErrorSender{tdpConn: tdpConn},
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
		startEvent := audit.makeSessionStart(err)
		s.record(ctx, recorder, startEvent)
		s.emit(ctx, startEvent)
		return trace.Wrap(err)
	}

	startEvent := audit.makeSessionStart(nil)
	startEvent.AllowUserCreation = createUsers

	s.record(ctx, recorder, startEvent)
	s.emit(ctx, startEvent)

	err = rdpc.Run(ctx)

	// ctx may have been canceled, so emit with a separate context
	endEvent := audit.makeSessionEnd(recordSession)
	s.record(context.Background(), recorder, endEvent)
	s.emit(context.Background(), endEvent)

	return trace.Wrap(err)
}

func (s *WindowsService) makeTDPSendHandler(
	ctx context.Context,
	recorder libevents.SessionPreparerRecorder,
	delay func() int64,
	tdpConn *tdp.Conn,
	audit *desktopSessionAuditor,
) func(m tdp.Message, b []byte) {
	return func(m tdp.Message, b []byte) {
		switch b[0] {
		case byte(tdp.TypeRDPConnectionInitialized), byte(tdp.TypeRDPFastPathPDU), byte(tdp.TypePNG2Frame),
			byte(tdp.TypePNGFrame), byte(tdp.TypeError), byte(tdp.TypeNotification):
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
				s.cfg.Logger.WarnContext(ctx, "refusing to record PNG frame, image too large", "len", len(b))
			} else {
				if err := libevents.SetupAndRecordEvent(ctx, recorder, e); err != nil {
					s.cfg.Logger.WarnContext(ctx, "could not record desktop recording event", "error", err)
				}
			}
		case byte(tdp.TypeClipboardData):
			if clip, ok := m.(tdp.ClipboardData); ok {
				// the TDP send handler emits a clipboard receive event, because we
				// received clipboard data from the remote desktop and are sending
				// it on the TDP connection
				rxEvent := audit.makeClipboardReceive(int32(len(clip)))
				s.emit(ctx, rxEvent)
			}
		case byte(tdp.TypeSharedDirectoryAcknowledge):
			if message, ok := m.(tdp.SharedDirectoryAcknowledge); ok {
				s.emit(ctx, audit.makeSharedDirectoryStart(message))
			}
		case byte(tdp.TypeSharedDirectoryReadRequest):
			if message, ok := m.(tdp.SharedDirectoryReadRequest); ok {
				errorEvent := audit.onSharedDirectoryReadRequest(message)
				if errorEvent != nil {
					// if we can't audit due to a full cache, abort the connection
					// as a security measure
					if err := tdpConn.Close(); err != nil {
						s.cfg.Logger.ErrorContext(ctx, "error when terminating session for audit cache maximum size violation", "session_id", audit.sessionID)
					}
					s.emit(ctx, errorEvent)
				}
			}
		case byte(tdp.TypeSharedDirectoryWriteRequest):
			if message, ok := m.(tdp.SharedDirectoryWriteRequest); ok {
				errorEvent := audit.onSharedDirectoryWriteRequest(message)
				if errorEvent != nil {
					// if we can't audit due to a full cache, abort the connection
					// as a security measure
					if err := tdpConn.Close(); err != nil {
						s.cfg.Logger.ErrorContext(ctx, "error when terminating session for audit cache maximum size violation", "session_id", audit.sessionID)
					}
					s.emit(ctx, errorEvent)
				}
			}
		}
	}
}

func (s *WindowsService) makeTDPReceiveHandler(
	ctx context.Context,
	recorder libevents.SessionPreparerRecorder,
	delay func() int64,
	tdpConn *tdp.Conn,
	audit *desktopSessionAuditor,
) func(m tdp.Message) {
	return func(m tdp.Message) {
		switch msg := m.(type) {
		case tdp.ClientScreenSpec, tdp.MouseButton, tdp.MouseMove:
			b, err := m.Encode()
			if err != nil {
				s.cfg.Logger.WarnContext(ctx, "could not emit desktop recording event", "error", err)
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
				s.cfg.Logger.WarnContext(ctx, "refusing to record message", "len", len(b), "type", fmt.Sprintf("%T", m))
			} else {
				if err := libevents.SetupAndRecordEvent(ctx, recorder, e); err != nil {
					s.cfg.Logger.WarnContext(ctx, "could not record desktop recording event", "error", err)
				}
			}
		case tdp.ClipboardData:
			// the TDP receive handler emits a clipboard send event, because we
			// received clipboard data from the user (over TDP) and are sending
			// it to the remote desktop
			sendEvent := audit.makeClipboardSend(int32(len(msg)))
			s.emit(ctx, sendEvent)
		case tdp.SharedDirectoryAnnounce:
			errorEvent := audit.onSharedDirectoryAnnounce(m.(tdp.SharedDirectoryAnnounce))
			if errorEvent != nil {
				// if we can't audit due to a full cache, abort the connection
				// as a security measure
				if err := tdpConn.Close(); err != nil {
					s.cfg.Logger.ErrorContext(ctx, "error when terminating session for audit cache maximum size violation",
						"session_id", audit.sessionID, "error", err)
				}
				s.emit(ctx, errorEvent)
			}
		case tdp.SharedDirectoryReadResponse:
			s.emit(ctx, audit.makeSharedDirectoryReadResponse(msg))
		case tdp.SharedDirectoryWriteResponse:
			s.emit(ctx, audit.makeSharedDirectoryWriteResponse(msg))
		}
	}
}

func (s *WindowsService) getServiceHeartbeatInfo() (types.Resource, error) {
	srv, err := types.NewWindowsDesktopServiceV3(
		types.Metadata{
			Name:   s.cfg.Heartbeat.HostUUID,
			Labels: s.cfg.Labels,
		},
		types.WindowsDesktopServiceSpecV3{
			Addr:            s.cfg.Heartbeat.PublicAddr,
			TeleportVersion: teleport.Version,
			Hostname:        s.cfg.Hostname,
			ProxyIDs:        s.cfg.ConnectedProxyGetter.GetProxyIDs(),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	srv.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	return srv, nil
}

// staticHostHeartbeatInfo generates the Windows Desktop resource
// for heartbeating statically defined hosts
func (s *WindowsService) staticHostHeartbeatInfo(host servicecfg.WindowsHost,
	getHostLabels func(string) map[string]string,
) func() (types.Resource, error) {
	return func() (types.Resource, error) {
		addr := host.Address.String()
		labels := getHostLabels(addr)
		for k, v := range host.Labels {
			labels[k] = v
		}
		name := host.Name
		if name == "" {
			var err error
			name, err = s.nameForStaticHost(addr)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		}
		labels[types.OriginLabel] = types.OriginConfigFile
		labels[types.ADLabel] = strconv.FormatBool(host.AD)
		desktop, err := types.NewWindowsDesktopV3(
			name,
			labels,
			types.WindowsDesktopSpecV3{
				Addr:   addr,
				Domain: s.cfg.Domain,
				HostID: s.cfg.Heartbeat.HostUUID,
				NonAD:  !host.AD,
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

// generateUserCert generates a keypair for the given Windows username,
// optionally querying LDAP for the user's Security Identifier.
func (s *WindowsService) generateUserCert(ctx context.Context, username string, ttl time.Duration, desktop types.WindowsDesktop, createUsers bool, groups []string) (certDER, keyDER []byte, err error) {
	var activeDirectorySID string
	if !desktop.NonAD() {
		// Find the user's SID
		filter := windows.CombineLDAPFilters([]string{
			fmt.Sprintf("(%s=%s)", windows.AttrSAMAccountType, windows.AccountTypeUser),
			fmt.Sprintf("(%s=%s)", windows.AttrSAMAccountName, username),
		})
		s.cfg.Logger.DebugContext(ctx, "querying LDAP for objectSid of Windows user", "username", username, "filter", filter)

		entries, err := s.lc.ReadWithFilter(s.cfg.LDAPConfig.DomainDN(), filter, []string{windows.AttrObjectSid})
		// if LDAP-based desktop discovery is not enabled, there may not be enough
		// traffic to keep the connection open. Attempt to open a new LDAP connection
		// in this case.
		if trace.IsConnectionProblem(err) {
			s.initializeLDAP() // ignore error, this is a best effort attempt
			entries, err = s.lc.ReadWithFilter(s.cfg.LDAPConfig.DomainDN(), filter, []string{windows.AttrObjectSid})
		}
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if len(entries) == 0 {
			return nil, nil, trace.NotFound("could not find Windows account %q", username)
		} else if len(entries) > 1 {
			s.cfg.Logger.WarnContext(ctx, "found multiple entries for user, taking the first", "username", username)
		}
		activeDirectorySID, err = windows.ADSIDStringFromLDAPEntry(entries[0])
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		s.cfg.Logger.DebugContext(ctx, "Found objectSid Windows user", "username", username)
	}
	return s.generateCredentials(ctx, generateCredentialsRequest{
		username:           username,
		domain:             desktop.GetDomain(),
		ttl:                ttl,
		activeDirectorySID: activeDirectorySID,
		createUser:         createUsers,
		groups:             groups,
	})
}

// generateCredentialsRequest are the request parameters for generating a windows cert/key pair
type generateCredentialsRequest struct {
	// username is the Windows username
	username string
	// domain is the Windows domain
	domain string
	// ttl for the certificate
	ttl time.Duration
	// activeDirectorySID is the SID of the Windows user
	// specified by Username. If specified (!= ""), it is
	// encoded in the certificate per https://go.microsoft.com/fwlink/?linkid=2189925.
	activeDirectorySID string
	// createUser specifies if Windows user should be created if missing
	createUser bool
	// groups are groups that user should be member of
	groups  []string
	omitCDP bool
}

// generateCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
func (s *WindowsService) generateCredentials(ctx context.Context, request generateCredentialsRequest) (certDER, keyDER []byte, err error) {
	// If PKI domain has been overridden, make sure we pass that through
	// to the cert request, otherwise the CRL in the cert we issue will
	// point at the wrong domain.
	lc := s.cfg.LDAPConfig
	if s.cfg.PKIDomain != "" {
		lc.Domain = s.cfg.PKIDomain
	}

	return windows.GenerateWindowsDesktopCredentials(ctx, &windows.GenerateCredentialsRequest{
		CAType:             types.UserCA,
		Username:           request.username,
		Domain:             request.domain,
		TTL:                request.ttl,
		ClusterName:        s.clusterName,
		ActiveDirectorySID: request.activeDirectorySID,
		LDAPConfig:         lc,
		AuthClient:         s.cfg.AuthClient,
		CreateUser:         request.createUser,
		Groups:             request.groups,
		OmitCDP:            request.omitCDP,
	})
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
		HostID:   s.cfg.Heartbeat.HostUUID,
	}

	s.cfg.Logger.DebugContext(ctx, "Creating session tracker", "session_id", sessionID)
	tracker, err := srv.NewSessionTracker(ctx, trackerSpec, s.cfg.AuthClient)
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		if err := tracker.UpdateExpirationLoop(ctx, s.cfg.Clock); err != nil {
			s.cfg.Logger.WarnContext(ctx, "Failed to update session tracker expiration", "session_id", sessionID, "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		if err := tracker.Close(s.closeCtx); err != nil {
			s.cfg.Logger.DebugContext(s.closeCtx, "Failed to close session tracker", "session_id", sessionID)
		}
	}()

	return nil
}

// monitorErrorSender implements the io.StringWriter
// interface in order to allow us to pass connection
// monitor disconnect messages back to the frontend
// over the tdp.Conn
type monitorErrorSender struct {
	tdpConn *tdp.Conn
}

func (m *monitorErrorSender) WriteString(s string) (n int, err error) {
	if err := m.tdpConn.SendNotification(s, tdp.SeverityError); err != nil {
		return 0, trace.Wrap(err, "sending TDP error message")
	}

	return len(s), nil
}
