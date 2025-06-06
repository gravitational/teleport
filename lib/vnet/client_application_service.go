// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package vnet

import (
	"cmp"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api"
	"github.com/gravitational/teleport/api/client/proto"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

// clientApplicationService implements the gRPC
// [vnetv1.ClientApplicationServiceServer] to expose functionality that requires
// a Teleport client to the VNet admin service running in another process.
type clientApplicationService struct {
	// opt-in to compilation errors if this doesn't implement
	// [vnetv1.ClientApplicationServiceServer]
	vnetv1.UnsafeClientApplicationServiceServer

	cfg *clientApplicationServiceConfig

	// networkStackInfo will receive any network stack info reported via
	// ReportNetworkStackInfo.
	networkStackInfo chan *vnetv1.NetworkStackInfo

	// appSignerMu protects appSignerCache.
	appSignerMu sync.Mutex
	// appSignerCache caches the crypto.Signer for each certificate issued by
	// ReissueAppCert so that SignForApp can later use that signer.
	//
	// Signers are never deleted from the map. When the cert expires, the local
	// proxy in the admin process will detect the cert expiry and call
	// ReissueAppCert, which will overwrite the signer for the app with a new
	// one.
	appSignerCache map[appKey]crypto.Signer

	// sshSigners is a cache containing [crypto.Signer]s keyed by SSH session
	// ID. This "session ID" is a concept only used here for retrieving a signer
	// previously associated with the same session, it is not some Teleport
	// session identifier.
	sshSigners *utils.FnCache
}

type clientApplicationServiceConfig struct {
	fqdnResolver          *fqdnResolver
	localOSConfigProvider *LocalOSConfigProvider
	clientApplication     ClientApplication
	homePath              string
	clock                 clockwork.Clock
}

func newClientApplicationService(cfg *clientApplicationServiceConfig) (*clientApplicationService, error) {
	sshSigners, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   time.Minute,
		Clock: cfg.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clientApplicationService{
		cfg:              cfg,
		networkStackInfo: make(chan *vnetv1.NetworkStackInfo, 1),
		appSignerCache:   make(map[appKey]crypto.Signer),
		sshSigners:       sshSigners,
	}, nil
}

// AuthenticateProcess implements [vnetv1.ClientApplicationServiceServer.AuthenticateProcess].
func (s *clientApplicationService) AuthenticateProcess(ctx context.Context, req *vnetv1.AuthenticateProcessRequest) (*vnetv1.AuthenticateProcessResponse, error) {
	log.DebugContext(ctx, "Received AuthenticateProcess request from admin process")
	if req.Version != api.Version {
		return nil, trace.BadParameter("version mismatch, user process version is %s, admin process version is %s",
			api.Version, req.Version)
	}
	if err := platformAuthenticateProcess(ctx, req); err != nil {
		log.ErrorContext(ctx, "Failed to authenticate process", "error", err)
		return nil, trace.Wrap(err, "authenticating process")
	}
	return &vnetv1.AuthenticateProcessResponse{
		Version: api.Version,
	}, nil
}

// ReportNetworkStackInfo implements [vnetv1.ClientApplicationServiceServer.ReportNetworkStackInfo].
func (s *clientApplicationService) ReportNetworkStackInfo(ctx context.Context, req *vnetv1.ReportNetworkStackInfoRequest) (*vnetv1.ReportNetworkStackInfoResponse, error) {
	select {
	case s.networkStackInfo <- req.GetNetworkStackInfo():
	default:
		return nil, trace.BadParameter("ReportNetworkStackInfo must be called exactly once")
	}
	return &vnetv1.ReportNetworkStackInfoResponse{}, nil
}

// Ping implements [vnetv1.ClientApplicationServiceServer.Ping].
func (s *clientApplicationService) Ping(ctx context.Context, req *vnetv1.PingRequest) (*vnetv1.PingResponse, error) {
	return &vnetv1.PingResponse{}, nil
}

// ResolveFQDN implements [vnetv1.ClientApplicationServiceServer.ResolveFQDN].
func (s *clientApplicationService) ResolveFQDN(ctx context.Context, req *vnetv1.ResolveFQDNRequest) (*vnetv1.ResolveFQDNResponse, error) {
	resp, err := s.cfg.fqdnResolver.ResolveFQDN(ctx, req.GetFqdn())
	return resp, trace.Wrap(err, "resolving FQDN")
}

// ReissueAppCert implements [vnetv1.ClientApplicationServiceServer.ReissueAppCert].
// It caches the signer issued for each app so that it can later be used to
// issue signatures in [clientApplicationService.SignForApp].
func (s *clientApplicationService) ReissueAppCert(ctx context.Context, req *vnetv1.ReissueAppCertRequest) (*vnetv1.ReissueAppCertResponse, error) {
	appInfo := req.GetAppInfo()
	if appInfo == nil {
		return nil, trace.BadParameter("missing AppInfo")
	}
	appKey := appInfo.GetAppKey()
	if err := checkAppKey(appKey); err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := s.cfg.clientApplication.ReissueAppCert(ctx, appInfo, uint16(req.GetTargetPort()))
	if err != nil {
		return nil, trace.Wrap(err, "reissuing app certificate")
	}
	s.setSignerForApp(appKey, uint16(req.GetTargetPort()), cert.PrivateKey.(crypto.Signer))
	return &vnetv1.ReissueAppCertResponse{
		Cert: cert.Certificate[0],
	}, nil
}

// SignForApp implements [vnetv1.ClientApplicationServiceServer.SignForApp].
// It uses a cached signer for the requested app, which must have previously
// been issued a certificate via [clientApplicationService.ReissueAppCert].
func (s *clientApplicationService) SignForApp(ctx context.Context, req *vnetv1.SignForAppRequest) (*vnetv1.SignForAppResponse, error) {
	signReq := req.GetSign()
	log.DebugContext(ctx, "Got SignForApp request",
		"app", req.GetAppKey(),
		"hash", signReq.GetHash(),
		"is_rsa_pss", signReq.PssSaltLength != nil,
		"pss_salt_len", signReq.GetPssSaltLength(),
		"digest_len", len(signReq.GetDigest()),
	)

	appKey := req.GetAppKey()
	if err := checkAppKey(appKey); err != nil {
		return nil, trace.Wrap(err)
	}
	signer, ok := s.getSignerForApp(req.GetAppKey(), uint16(req.GetTargetPort()))
	if !ok {
		return nil, trace.BadParameter("no signer for app %v", appKey)
	}

	signature, err := sign(signer, signReq)
	if err != nil {
		return nil, trace.Wrap(err, "signing for app %v", appKey)
	}
	return &vnetv1.SignForAppResponse{
		Signature: signature,
	}, nil
}

func sign(signer crypto.Signer, signReq *vnetv1.SignRequest) ([]byte, error) {
	var hash crypto.Hash
	switch signReq.GetHash() {
	case vnetv1.Hash_HASH_NONE:
		hash = crypto.Hash(0)
	case vnetv1.Hash_HASH_SHA256:
		hash = crypto.SHA256
	default:
		return nil, trace.BadParameter("unsupported hash %v", signReq.GetHash())
	}
	opts := crypto.SignerOpts(hash)
	if signReq.PssSaltLength != nil {
		opts = &rsa.PSSOptions{
			Hash:       hash,
			SaltLength: int(*signReq.PssSaltLength),
		}
	}
	signature, err := signer.Sign(rand.Reader, signReq.GetDigest(), opts)
	return signature, trace.Wrap(err)
}

func (s *clientApplicationService) setSignerForApp(appKey *vnetv1.AppKey, targetPort uint16, signer crypto.Signer) {
	s.appSignerMu.Lock()
	defer s.appSignerMu.Unlock()
	s.appSignerCache[newAppKey(appKey, targetPort)] = signer
}

func (s *clientApplicationService) getSignerForApp(appKey *vnetv1.AppKey, targetPort uint16) (crypto.Signer, bool) {
	s.appSignerMu.Lock()
	defer s.appSignerMu.Unlock()
	signer, ok := s.appSignerCache[newAppKey(appKey, targetPort)]
	return signer, ok
}

// OnNewConnection gets called whenever a new connection is about to be
// established through VNet for observability.
func (s *clientApplicationService) OnNewConnection(ctx context.Context, req *vnetv1.OnNewConnectionRequest) (*vnetv1.OnNewConnectionResponse, error) {
	if err := s.cfg.clientApplication.OnNewConnection(ctx, req.GetAppKey()); err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetv1.OnNewConnectionResponse{}, nil
}

// OnInvalidLocalPort gets called before VNet refuses to handle a connection
// to a multi-port TCP app because the provided port does not match any of the
// TCP ports in the app spec.
func (s *clientApplicationService) OnInvalidLocalPort(ctx context.Context, req *vnetv1.OnInvalidLocalPortRequest) (*vnetv1.OnInvalidLocalPortResponse, error) {
	s.cfg.clientApplication.OnInvalidLocalPort(ctx, req.GetAppInfo(), uint16(req.GetTargetPort()))
	return &vnetv1.OnInvalidLocalPortResponse{}, nil
}

// appKey is a clone of [vnetv1.AppKey] that is not a protobuf type so it can be
// used as a key in maps.
type appKey struct {
	profile, leafCluster, app string
	port                      uint16
}

func newAppKey(protoAppKey *vnetv1.AppKey, port uint16) appKey {
	return appKey{
		profile:     protoAppKey.GetProfile(),
		leafCluster: protoAppKey.GetLeafCluster(),
		app:         protoAppKey.GetName(),
		port:        port,
	}
}

// GetTargetOSConfiguration returns the configuration values that should be
// configured in the OS, including DNS zones that should be handled by the VNet
// DNS nameserver and the IPv4 CIDR ranges that should be routed to the VNet TUN
// interface.
func (s *clientApplicationService) GetTargetOSConfiguration(ctx context.Context, _ *vnetv1.GetTargetOSConfigurationRequest) (*vnetv1.GetTargetOSConfigurationResponse, error) {
	targetConfig, err := s.cfg.localOSConfigProvider.GetTargetOSConfiguration(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting target OS configuration")
	}
	return &vnetv1.GetTargetOSConfigurationResponse{
		TargetOsConfiguration: targetConfig,
	}, nil
}

// UserTLSCert returns the user TLS certificate for a specific profile.
func (s *clientApplicationService) UserTLSCert(ctx context.Context, req *vnetv1.UserTLSCertRequest) (*vnetv1.UserTLSCertResponse, error) {
	tlsCert, err := s.cfg.clientApplication.UserTLSCert(ctx, req.GetProfile())
	if err != nil {
		return nil, trace.Wrap(err, "getting user TLS cert")
	}
	if len(tlsCert.Certificate) == 0 {
		return nil, trace.Errorf("user TLS cert has no certificate")
	}
	dialOpts, err := s.cfg.clientApplication.GetDialOptions(ctx, req.GetProfile())
	if err != nil {
		return nil, trace.Wrap(err, "getting TLS dial options")
	}
	return &vnetv1.UserTLSCertResponse{
		Cert:        tlsCert.Certificate[0],
		DialOptions: dialOpts,
	}, nil
}

// SignForUserTLS signs a digest with the user TLS private key.
func (s *clientApplicationService) SignForUserTLS(ctx context.Context, req *vnetv1.SignForUserTLSRequest) (*vnetv1.SignForUserTLSResponse, error) {
	tlsCert, err := s.cfg.clientApplication.UserTLSCert(ctx, req.GetProfile())
	if err != nil {
		return nil, trace.Wrap(err, "getting user TLS config")
	}
	signer, ok := tlsCert.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, trace.Errorf("user TLS private key does not implement crypto.Signer")
	}
	signature, err := sign(signer, req.GetSign())
	if err != nil {
		return nil, trace.Wrap(err, "signing for user TLS certificate")
	}
	return &vnetv1.SignForUserTLSResponse{
		Signature: signature,
	}, nil
}

// SessionSSHConfig returns user SSH configuration values for an SSH session.
func (s *clientApplicationService) SessionSSHConfig(ctx context.Context, req *vnetv1.SessionSSHConfigRequest) (*vnetv1.SessionSSHConfigResponse, error) {
	clusterClient, err := s.cfg.clientApplication.GetCachedClient(ctx, req.GetProfile(), req.GetLeafCluster())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If req.LeafCluster is not empty the node is in the leaf cluster, else it
	// is in the root cluster.
	targetCluster := cmp.Or(req.GetLeafCluster(), req.GetRootCluster())
	target := client.NodeDetails{
		Addr:    req.GetAddress(),
		Cluster: targetCluster,
	}
	keyRing, completedMFA, err := clusterClient.SessionSSHKeyRing(ctx, req.GetUser(), target)
	if err != nil {
		return nil, trace.Wrap(err, "getting KeyRing for SSH session")
	}
	if !completedMFA && keyRing.Cert == nil && targetCluster == req.GetLeafCluster() {
		// It's possible/likely the user doesn't have an SSH cert specifically
		// for the leaf cluster. Luckily if MFA was not required, the root
		// cluster cert should work.
		log.DebugContext(ctx, "Leaf cluster KeyRing had no SSH cert, using root cluster KeyRing")
		rootClusterClient, err := s.cfg.clientApplication.GetCachedClient(ctx, req.GetProfile(), "")
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Set the target cluster to the root cluster and disable the MFA check
		// so that SessionSSHKeyRing will just return the base root cluster
		// keyring.
		target.Cluster = req.GetRootCluster()
		target.MFACheck = &proto.IsMFARequiredResponse{
			Required:    false,
			MFARequired: proto.MFARequired_MFA_REQUIRED_NO,
		}
		keyRing, _, err = rootClusterClient.SessionSSHKeyRing(ctx, req.GetUser(), target)
		if err != nil {
			return nil, trace.Wrap(err, "getting root cluster KeyRing for SSH session")
		}
	}
	if len(keyRing.Cert) == 0 {
		return nil, trace.Errorf("user KeyRing has no SSH cert")
	}
	sshCert, _, _, _, err := ssh.ParseAuthorizedKey(keyRing.Cert)
	if err != nil {
		return nil, trace.Wrap(err, "parsing user SSH certificate")
	}
	var trustedCAs [][]byte
	for _, trustedCert := range keyRing.TrustedCerts {
		if trustedCert.ClusterName != targetCluster {
			continue
		}
		for _, authorizedKey := range trustedCert.AuthorizedKeys {
			trustedCA, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKey)
			if err != nil {
				return nil, trace.Wrap(err, "parsing CA cert")
			}
			trustedCAs = append(trustedCAs, trustedCA.Marshal())
		}
	}
	if len(trustedCAs) == 0 {
		return nil, trace.Errorf("user KeyRing host no trusted SSH CAs for cluster %s", targetCluster)
	}
	sessionID := s.setSignerForSSHSession(keyRing.SSHPrivateKey)
	return &vnetv1.SessionSSHConfigResponse{
		SessionId:  sessionID,
		Cert:       sshCert.Marshal(),
		TrustedCas: trustedCAs,
	}, nil
}

// SignForSSHSession signs a digest with the SSH private key associated with the
// session from a previous call to SessionSSHConfig.
func (s *clientApplicationService) SignForSSHSession(ctx context.Context, req *vnetv1.SignForSSHSessionRequest) (*vnetv1.SignForSSHSessionResponse, error) {
	signer, err := s.getSignerForSSHSession(ctx, req.GetSessionId())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signature, err := sign(signer, req.GetSign())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetv1.SignForSSHSessionResponse{
		Signature: signature,
	}, nil
}

func (s *clientApplicationService) setSignerForSSHSession(signer crypto.Signer) string {
	sessionID := uuid.NewString()
	s.sshSigners.Set(sessionID, signer)
	return sessionID
}

func (s *clientApplicationService) getSignerForSSHSession(ctx context.Context, sessionID string) (crypto.Signer, error) {
	signer, err := utils.FnCacheGet(ctx, s.sshSigners, sessionID, func(ctx context.Context) (crypto.Signer, error) {
		return nil, trace.NotFound("session key expired")
	})
	return signer, trace.Wrap(err)
}

// ExchangeSSHKeys recevies the VNet service host CA public key and writes it to
// ${TELEPORT_HOME}/vnet_known_hosts so that third-party SSH clients can trust
// it. It then reads or generates ${TELEPORT_HOME}/id_vnet(.pub) which SSH
// clients should be configured to use for connections to VNet SSH. It returns
// id_vnet.pub so that VNet SSH can trust it for incoming connections.
func (s *clientApplicationService) ExchangeSSHKeys(ctx context.Context, req *vnetv1.ExchangeSSHKeysRequest) (*vnetv1.ExchangeSSHKeysResponse, error) {
	hostPublicKey, err := ssh.ParsePublicKey(req.GetHostPublicKey())
	if err != nil {
		return nil, trace.Wrap(err, "parsing host public key")
	}
	userPublicKey, err := writeSSHKeys(s.cfg.homePath, hostPublicKey)
	if err != nil {
		return nil, trace.Wrap(err, "writing SSH keys")
	}
	return &vnetv1.ExchangeSSHKeysResponse{
		UserPublicKey: userPublicKey.Marshal(),
	}, nil
}

// checkAppKey checks that at least the app profile and name are set, which are
// necessary to to disambiguate apps. LeafCluster is expected to be empty if the
// app is in a root cluster.
func checkAppKey(key *vnetv1.AppKey) error {
	switch {
	case key == nil:
		return trace.BadParameter("app key must not be nil")
	case key.GetProfile() == "":
		return trace.BadParameter("app key profile must be set")
	case key.GetName() == "":
		return trace.BadParameter("app key name must be set")
	}
	return nil
}
