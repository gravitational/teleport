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
	"strings"
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
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// dnsDialTimeout is the timeout for dialing the LDAP server
	// when resolving Windows Desktop hostnames
	dnsDialTimeout = 5 * time.Second

	// windowsDesktopCertTTL is the TTL for Teleport-issued Windows Certificates.
	// Certificates are requested on each connection attempt, so the TTL is
	// deliberately set to a small value to give enough time to establish a
	// single desktop session.
	windowsDesktopCertTTL = 5 * time.Minute
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
	Clock clockwork.Clock
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
	// TODO(zmb3): add support for LDAP filters as defined in RFD #34
}

// LDAPConfig contains parameters for connecting to an LDAP server.
type LDAPConfig struct {
	// Addr is the LDAP server address in the form host:port.
	// Standard port is 389.
	Addr string
	// Domain is an Active Directory domain name, like "example.com".
	Domain string
	// Username is an LDAP username, like "EXAMPLE\Administrator", where
	// "EXAMPLE" is the NetBIOS version of Domain.
	Username string
	// Password is an LDAP password. Usually it's the same user password used
	// for local logins on the Domain Controller.
	Password string
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
	if cfg.Password == "" {
		return trace.BadParameter("missing Password in LDAPConfig")
	}
	return nil
}

// ldapPath is a helper type representing a hierarchical path in LDAP.
type ldapPath []string

func (cfg LDAPConfig) dn(path ldapPath) string {
	// Here's an example DN:
	//
	// CN=mycluster,CN=Certification Authorities,CN=Public Key Services,CN=Services,CN=Configuration,DC=example,DC=com
	//
	// You read it backwards:
	// - DC=example,DC=com means "example.com" domain
	// - CN=mycluster,CN=Certification Authorities,CN=Public Key Services,CN=Services,CN=Configuration
	//   means "Configuration/Services/Public Key Services/Certification Authorities/mycluster"
	//   entry, where "mycluster" is the Teleport cluster name
	//
	// The path argument is expected to be a normal hierarchical path, like:
	// ["Configuration", "Services", "Public Key Services", "Certification Authorities", "mycluster"]
	s := new(strings.Builder)
	for i := len(path) - 1; i >= 0; i-- {
		fmt.Fprintf(s, "CN=%s", path[i])
		if i != 0 {
			s.WriteRune(',')
		}
	}
	for _, dc := range strings.Split(cfg.Domain, ".") {
		// we always want to emit a leading ',' unless path was empty
		if s.Len() > 0 {
			s.WriteString(",")
		}
		fmt.Fprintf(s, "DC=%s", dc)
	}
	return s.String()
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

	clusterName, err := cfg.AccessPoint.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "fetching cluster name: %v", err)
	}

	lc, err := newLDAPClient(cfg.LDAPConfig)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to LDAP server: %v", err)
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
		lc:          lc,
		clusterName: clusterName.GetClusterName(),
		closeCtx:    ctx,
		close:       close,
	}

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
	// TODO(zmb3): do these periodically
	if err := s.updateCA(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(zmb3): session recording.

	if err := s.startServiceHeartbeat(); err != nil {
		s.Close()
		return nil, trace.Wrap(err)
	}

	if err := s.startStaticHostHeartbeats(); err != nil {
		s.Close()
		return nil, trace.Wrap(err)
	}

	// for now, the only valid base DN is '*'
	// TODO(zmb3): allow further customizing the search
	if s.cfg.DiscoveryBaseDN == types.Wildcard {
		if err := s.startDiscoveredHostHeartbeats(); err != nil {
			s.Close()
			return nil, trace.Wrap(err)
		}
	} else if len(s.cfg.Heartbeat.StaticHosts) == 0 {
		s.cfg.Log.Warnln("desktop discovery via LDAP is disabled, and no hosts are defined in the configuration; there will be no Windows desktops available to connect")
	} else {
		s.cfg.Log.Infoln("desktop discovery via LDAP is disabled, set 'base_dn: *' to enable")
	}

	return s, nil
}

func (s *WindowsService) startServiceHeartbeat() error {
	heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
		Context:         s.closeCtx,
		Component:       teleport.ComponentWindowsDesktop,
		Mode:            srv.HeartbeatModeWindowsDesktopService,
		Announcer:       s.cfg.AccessPoint,
		GetServerInfo:   s.getServiceHeartbeatInfo,
		KeepAlivePeriod: apidefaults.ServerKeepAliveTTL,
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

// computerAttributes are the attributes we fetch when discovering
// Windows hosts via LDAP
// see: https://docs.microsoft.com/en-us/windows/win32/adschema/c-computer#windows-server-2012-attributes
var computerAttribtes = []string{
	attrName,
	attrDNSHostName,
	attrObjectGUID,
	attrOS,
	attrOSVersion,
}

const (
	// computerClass is the object class for computers in Active Directory
	computerClass = "computer"

	attrName        = "name"
	attrDNSHostName = "dNSHostName" // unusual capitalization is correct
	attrObjectGUID  = "objectGUID"
	attrOS          = "operatingSystem"
	attrOSVersion   = "operatingSystemVersion"
)

// startDiscoveredHostHeartbeats kicks off background processing to discover Windows
// hosts via LDAP and perform heartbeats to record them with the auth server
func (s *WindowsService) startDiscoveredHostHeartbeats() error {
	// here we're searching for computers using an empty ldapPath
	// in order to search from the root
	entries, err := s.lc.read(ldapPath{}, computerClass, computerAttribtes)
	if err != nil {
		return trace.Wrap(err)
	}

	s.cfg.Log.Infof("discovered %d Windows hosts", len(entries))

	// NOTE: for now, we only search for hosts once, and then kick off a heartbeat
	// for each discovered host.
	//
	// TODO(zmb3): periodically refresh hosts from LDAP in order to detect new hosts
	// and stop heartbeating for hosts that no longer exist
	// (this may require updates to srv.Heartbeat)
	for _, entry := range entries {
		heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
			Context:   s.closeCtx,
			Component: teleport.ComponentWindowsDesktop,
			Mode:      srv.HeartbeatModeWindowsDesktop,
			Announcer: s.cfg.AccessPoint,
			GetServerInfo: func() (types.Resource, error) {
				return s.dynamicHostHeartbeatInfo(s.closeCtx, entry, s.cfg.HostLabelsFn)
			},
			// Larger than normal periods are due to the fact that we don't currently refresh
			// the list of hosts from LDAP. Since the heartbeat data is static we don't need
			// to announce it as frequently as we do for dynamic resources.
			// TODO(zmb3): reconsider timeouts when #8644 is addressed
			KeepAlivePeriod: apidefaults.ServerKeepAliveTTL * 2,
			AnnouncePeriod:  apidefaults.ServerAnnounceTTL + utils.RandomDuration(apidefaults.ServerAnnounceTTL/10),
			CheckPeriod:     defaults.HeartbeatCheckPeriod * 60,
			ServerTTL:       apidefaults.ServerAnnounceTTL,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		go func(entry *ldap.Entry) {
			if err := heartbeat.Run(); err != nil {
				s.cfg.Log.WithError(err).Errorf("heartbeat for Windows host %v ended with error", entry.DN)
			}
		}(entry)
	}

	return nil
}

// dynamicHostHeartbeatInfo generates the Windows Desktop resource
// for heartbeating hosts discovered via LDAP
func (s *WindowsService) dynamicHostHeartbeatInfo(ctx context.Context, entry *ldap.Entry, getHostLabels func(string) map[string]string) (types.Resource, error) {
	hostname := entry.GetAttributeValue(attrDNSHostName)

	labels := getHostLabels(hostname)
	labels["teleport.dev/dns_host_name"] = hostname
	labels["teleport.dev/computer_name"] = entry.GetAttributeValue(attrName)
	labels["teleport.dev/os"] = entry.GetAttributeValue(attrOS)
	labels["teleport.dev/os_version"] = entry.GetAttributeValue(attrOSVersion)
	labels["teleport.dev/windows_domain"] = s.cfg.Domain
	labels[types.OriginLabel] = types.OriginDynamic

	addrs, err := s.dnsResolver.LookupHost(ctx, hostname)
	if err != nil || len(addrs) == 0 {
		return nil, trace.WrapWithMessage(err, "couldn't resolve %q", hostname)
	}

	s.cfg.Log.Debugf("resolved %v => %v", hostname, addrs)
	addr, err := utils.ParseHostPortAddr(addrs[0], defaults.RDPListenPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktop, err := types.NewWindowsDesktopV3(
		// ensure no '.' in name, because we use SNI to route to the right
		// desktop, and our cert is valid for *.desktop.teleport.cluster.local
		strings.ReplaceAll(hostname, ".", "-"),
		labels,
		types.WindowsDesktopSpecV3{
			Addr:   addr.String(),
			Domain: s.cfg.Domain,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	desktop.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
	return desktop, nil
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
			KeepAlivePeriod: apidefaults.ServerKeepAliveTTL,
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

		go s.handleConnection(conn)
	}
}

func (s *WindowsService) handleConnection(con net.Conn) {
	defer con.Close()
	log := s.cfg.Log

	// Check connection limits.
	remoteAddr, _, err := net.SplitHostPort(con.RemoteAddr().String())
	if err != nil {
		log.WithError(err).Errorf("Could not parse client IP from %q", con.RemoteAddr().String())
		return
	}
	log = log.WithField("client-ip", remoteAddr)
	if err := s.cfg.ConnLimiter.AcquireConnection(remoteAddr); err != nil {
		log.WithError(err).Warning("Connection limit exceeded, rejecting connection")
		return
	}
	defer s.cfg.ConnLimiter.ReleaseConnection(remoteAddr)

	// Authenticate the client.
	tlsConn, ok := con.(*tls.Conn)
	if !ok {
		log.Errorf("Got %T from TLS listener, expected *tls.Conn", con)
		return
	}
	ctx, err := s.middleware.WrapContextWithUser(s.closeCtx, tlsConn)
	if err != nil {
		log.WithError(err).Warning("mTLS authentication failed for incoming connection")
		return
	}
	log.Debug("Authenticated Windows desktop connection")

	authContext, err := s.cfg.Authorizer.Authorize(ctx)
	if err != nil {
		log.WithError(err).Warning("authorization failed for Windows desktop connection")
		return
	}

	// Fetch the target desktop info. UUID of the desktop is passed via SNI.
	desktopUUID := strings.TrimSuffix(tlsConn.ConnectionState().ServerName, SNISuffix)
	log = log.WithField("desktop-uuid", desktopUUID)

	desktop, err := s.cfg.AccessPoint.GetWindowsDesktop(ctx, desktopUUID)
	if err != nil {
		log.WithError(err).Warning("Failed to fetch desktop by UUID")
		return
	}

	log = log.WithField("desktop-addr", desktop.GetAddr())
	log.Debug("Connecting to Windows desktop")
	defer log.Debug("Windows desktop disconnected")

	if err := s.connectRDP(ctx, log, tlsConn, desktop, authContext); err != nil {
		log.WithError(err).Error("RDP connection failed")
		return
	}
}

func (s *WindowsService) connectRDP(ctx context.Context, log logrus.FieldLogger, conn net.Conn, desktop types.WindowsDesktop, authCtx *auth.Context) error {
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

	var windowsUser string
	authorize := func(login string) error {
		windowsUser = login // capture attempted login user
		return authCtx.Checker.CheckAccess(
			desktop,
			services.AccessMFAParams{Verified: true},
			services.NewWindowsLoginMatcher(login))
	}

	tdpConn := tdp.NewConn(conn)
	rdpc, err := rdpclient.New(ctx, rdpclient.Config{
		Log: log,
		GenerateUserCert: func(ctx context.Context, username string) (certDER, keyDER []byte, err error) {
			return s.generateCredentials(ctx, username, desktop.GetDomain())
		},
		Addr:          desktop.GetAddr(),
		InputMessage:  tdpConn.InputMessage,
		OutputMessage: tdpConn.OutputMessage,
		AuthorizeFn:   authorize,
	})
	if err != nil {
		s.onSessionStart(ctx, &identity, windowsUser, string(sessionID), desktop, err)
		return trace.Wrap(err)
	}

	monitorCfg := srv.MonitorConfig{
		Context:           ctx,
		Conn:              conn,
		Clock:             s.cfg.Clock,
		ClientIdleTimeout: authCtx.Checker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout()),
		Entry:             log,
		Emitter:           s.cfg.Emitter,
		LockWatcher:       s.cfg.LockWatcher,
		LockTargets:       services.LockTargetsFromTLSIdentity(identity),
		Tracker:           rdpc,
		TeleportUser:      identity.Username,
		ServerID:          desktop.GetName(),
	}
	shouldDisconnectExpiredCert := authCtx.Checker.AdjustDisconnectExpiredCert(authPref.GetDisconnectExpiredCert())
	if shouldDisconnectExpiredCert && !identity.Expires.IsZero() {
		monitorCfg.DisconnectExpiredCert = identity.Expires
	}

	if err := srv.StartMonitor(monitorCfg); err != nil {
		// if we can't establish a connection monitor then we can't enforce RBAC.
		// consider this a connection failure and return an error
		// (in the happy path, rdpc remains open until Wait() completes)
		rdpc.Close()
		s.onSessionStart(ctx, &identity, windowsUser, string(sessionID), desktop, err)
		return trace.Wrap(err)
	}

	s.onSessionStart(ctx, &identity, windowsUser, string(sessionID), desktop, nil)
	err = rdpc.Wait()
	s.onSessionEnd(ctx, &identity, windowsUser, string(sessionID), desktop)

	return trace.Wrap(err)
}

func (s *WindowsService) getServiceHeartbeatInfo() (types.Resource, error) {
	srv, err := types.NewWindowsDesktopServiceV3(
		s.cfg.Heartbeat.HostUUID,
		types.WindowsDesktopServiceSpecV3{
			Addr:            s.cfg.Heartbeat.PublicAddr,
			TeleportVersion: teleport.Version,
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
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		desktop.SetExpiry(s.cfg.Clock.Now().UTC().Add(apidefaults.ServerAnnounceTTL))
		return desktop, nil
	}
}

// nameForStaticHost attempts to find the UUID of an existing Windows desktop
// with the same address. If no matching address is found, a new UUID is
// generated.
//
// The list of WindowsDesktop objects should be read from the local cache. It
// should be reasonably fast to do this scan on every heartbeat. However, with
// a very large number of desktops in the cluster, this may use up a lot of CPU
// time.
//
// TODO(zmb3): think of an alternative way to not duplicate desktop objects
// coming from different windows_desktop_services.
func (s *WindowsService) nameForStaticHost(addr string) (string, error) {
	desktops, err := s.cfg.AccessPoint.GetWindowsDesktops(s.closeCtx)
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

func (s *WindowsService) updateCA(ctx context.Context) error {
	// Publish the CA cert for current cluster CA. For trusted clusters, their
	// respective windows_desktop_services will publish their CAs so we don't
	// have to do it here.
	//
	// TODO(zmb3): support multiple CA certs per cluster (such as with HSMs).
	ca, err := s.cfg.AccessPoint.GetCertAuthority(types.CertAuthID{
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

func (s *WindowsService) updateCAInNTAuthStore(ctx context.Context, caDER []byte) error {
	// Check if our CA is already in the store. The LDAP entry for NTAuth store
	// is constant and it should always exist.
	ntauthPath := ldapPath{"Configuration", "Services", "Public Key Services", "NTAuthCertificates"}
	entries, err := s.lc.read(ntauthPath, "certificationAuthority", []string{"cACertificate"})
	if err != nil {
		return trace.Wrap(err, "fetching existing CAs: %v", err)
	}
	if len(entries) != 1 {
		return trace.BadParameter("expected exactly 1 NTAuthCertificates CA store at %q, but found %d", ntauthPath, len(entries))
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

	// CA is not in the store, append it.
	updatedCAs := make([]string, 0, len(existingCAs)+1)
	for _, existingCADER := range existingCAs {
		updatedCAs = append(updatedCAs, string(existingCADER))
	}
	updatedCAs = append(updatedCAs, string(caDER))

	if err := s.lc.update(ntauthPath, map[string][]string{
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
	containerPath := ldapPath{"Configuration", "Services", "Public Key Services", "CDP", "Teleport"}
	crlPath := append(containerPath, s.clusterName)

	// Create the parent container.
	if err := s.lc.createContainer(containerPath); err != nil {
		return trace.Wrap(err, "creating CRL container: %v", err)
	}

	// Create the CRL object itself.
	if err := s.lc.create(
		crlPath,
		"cRLDistributionPoint",
		map[string][]string{"certificateRevocationList": {string(crlDER)}},
	); err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// CRL already exists, update it.
		if err := s.lc.update(
			crlPath,
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

// generateCredentials generates a private key / certificate pair for the given
// Windows username. The certificate has certain special fields different from
// the regular Teleport user certificate, to meet the requirements of Active
// Directory. See:
// https://docs.microsoft.com/en-us/windows/security/identity-protection/smart-cards/smart-card-certificate-requirements-and-enumeration
func (s *WindowsService) generateCredentials(ctx context.Context, username, domain string) (certDER, keyDER []byte, err error) {
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
			extKeyUsageExtension,
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
	//
	// In other words, the domain var below may not be the same as
	// s.cfg.LDAPConfig.Domain and that's expected.
	crlPath := ldapPath{"Configuration", "Services", "Public Key Services", "CDP", "Teleport", s.clusterName}
	crlDN := s.cfg.LDAPConfig.dn(crlPath)
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
		TTL:         proto.Duration(windowsDesktopCertTTL),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	certBlock, _ := pem.Decode(genResp.Cert)
	certDER = certBlock.Bytes
	return certDER, keyDER, nil
}

var extKeyUsageExtension = pkix.Extension{
	Id: asn1.ObjectIdentifier{2, 5, 29, 37}, // Extended Key Usage OID.
	Value: func() []byte {
		val, err := asn1.Marshal([]asn1.ObjectIdentifier{
			{1, 3, 6, 1, 5, 5, 7, 3, 2},       // Client Authentication OID.
			{1, 3, 6, 1, 4, 1, 311, 20, 2, 2}, // Smartcard Logon OID.
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
	ext := pkix.Extension{Id: asn1.ObjectIdentifier{2, 5, 29, 17}}
	var err error
	ext.Value, err = asn1.Marshal(
		subjectAltName{
			OtherName: otherName{
				OID: asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 3}, // UPN OID
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
