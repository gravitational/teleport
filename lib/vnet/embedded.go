// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"errors"
	"io"
	"sync"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/tun"
	"google.golang.org/grpc"
)

// EmbeddedVNetConfig is the configuration for an embeded/single-process
// application of vnet. It currently only supports TCP app access, not SSH.
type EmbeddedVNetConfig struct {
	// Device is the TUN device vnet will bind to.
	Device tun.Device

	// ApplicationService is used to provide configuration, resolve DNS queries,
	// and generate client certificates.
	ApplicationService EmbeddedApplicationService

	// ConfigureHost is called to update the host's routing table and DNS
	// resolver configuration.
	ConfigureHost func(ctx context.Context, cfg EmbeddedVNetHostConfig) error
}

// EmbeddedApplicationService is a subset of vnetv1.ClientApplicationServiceClient
// required for embedded/single-process applications of vnet.
type EmbeddedApplicationService interface {
	// ResolveFQDN is called during DNS resolution to resolve a fully-qualified
	// domain name to a target.
	ResolveFQDN(ctx context.Context, req *vnetv1.ResolveFQDNRequest) (*vnetv1.ResolveFQDNResponse, error)

	// GetTargetOSConfiguration gets the target OS configuration.
	GetTargetOSConfiguration(ctx context.Context, in *vnetv1.GetTargetOSConfigurationRequest) (*vnetv1.GetTargetOSConfigurationResponse, error)

	// GetAppCert issues a TLS certificate for the given application. The result
	// will be cached so that its private key can be used for signing.
	GetAppCert(ctx context.Context, req *vnetv1.ReissueAppCertRequest) (*tls.Certificate, error)

	// GetUserCert gets the user's TLS certificate. The result will be cached so
	// that its private key can be used for signing.
	GetUserCert(ctx context.Context, req *vnetv1.UserTLSCertRequest) (*tls.Certificate, error)
}

// EmbeddedVNetHostConfig is passed to EmbeddedVNetConfig.ConfigureHost to
// reconfigure the host's routing table and DNS resolver.
type EmbeddedVNetHostConfig struct {
	// DeviceIPv4 is the desired IPv4 address of the TUN device.
	DeviceIPv4 string

	// DeviceIPv6 is the desired IPv6 address of the TUN device.
	DeviceIPv6 string

	// CIDRRanges are the desired IPv4 and IPv6 CIDR ranges that should be
	// routed to the TUN device.
	CIDRRanges []string

	// DNSAddrs are the IPv4 and IPv6 addresses for the vnet's nameserver.
	DNSAddrs []string

	// DNSZones are the zones for which the host's DNS resolver should send
	// queries to the vnet's nameserver.
	DNSZones []string
}

// EmbeddedVNet is an embeded/single-process application of vnet.
type EmbeddedVNet struct {
	device        tun.Device
	client        *clientApplicationServiceClient
	configureHost func(context.Context, EmbeddedVNetHostConfig) error
}

// NewEmbeddedVNet creates an embedded vnet with the given configuration.
//
// It is the caller's responsibility to configure the host by creating the TUN
// device, updating the OS routing table, and configuring the DNS resolver.
func NewEmbeddedVNet(cfg EmbeddedVNetConfig) (*EmbeddedVNet, error) {
	if cfg.Device == nil {
		return nil, trace.BadParameter("Device is required")
	}
	if cfg.ApplicationService == nil {
		return nil, trace.BadParameter("ApplicationService is required")
	}
	if cfg.ConfigureHost == nil {
		return nil, trace.BadParameter("ConfigureHost is required")
	}
	return &EmbeddedVNet{
		device: cfg.Device,
		client: &clientApplicationServiceClient{
			clt: &embeddedApplicationServiceClient{
				service:    cfg.ApplicationService,
				appSigners: make(map[appKey]crypto.Signer),
			},
			closer: io.NopCloser(bytes.NewReader([]byte{})),
		},
		configureHost: cfg.ConfigureHost,
	}, nil
}

// Run the vnet until the given context is cancelled.
func (vnet *EmbeddedVNet) Run(ctx context.Context) error {
	stackConfig, err := newNetworkStackConfig(ctx, vnet.device, vnet.client)
	if err != nil {
		return trace.Wrap(err, "creating network stack config")
	}

	stack, err := newNetworkStack(stackConfig)
	if err != nil {
		return trace.Wrap(err, "creating network stack")
	}

	tunName, err := vnet.device.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN device name")
	}

	osConfigProvider, err := newOSConfigProvider(osConfigProviderConfig{
		clt:           vnet.client,
		tunName:       tunName,
		ipv6Prefix:    stackConfig.ipv6Prefix.String(),
		dnsIPv6:       stackConfig.dnsIPv6.String(),
		addDNSAddress: stack.addDNSAddress,
	})
	if err != nil {
		return trace.Wrap(err, "creating OS config provider")
	}

	osConfigurator := newOSConfigurator(osConfigProvider,
		func(ctx context.Context, oc *osConfig, _ *osConfigState) error {
			// TODO: This feels a little hacky, think of something better.
			//
			// This function is called by the OS Configurator with an empty
			// oc to "deconfigure" the OS (clean up any previous state), which
			// we don't actually need for tbot.
			if oc.tunName == "" {
				return nil
			}

			return vnet.configureHost(ctx, EmbeddedVNetHostConfig{
				DeviceIPv4: oc.tunIPv4,
				DeviceIPv6: oc.tunIPv6,
				// TODO: Is this the cleanest way to configure the IPv6 route?
				CIDRRanges: append(oc.cidrRanges, oc.tunIPv6+"/64"),
				DNSAddrs:   oc.dnsAddrs,
				DNSZones:   oc.dnsZones,
			})
		},
	)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer log.InfoContext(ctx, "Network stack terminated.")
		if err := stack.run(ctx); err != nil {
			return trace.Wrap(err, "running network stack")
		}
		return errors.New("network stack terminated")
	})
	g.Go(func() error {
		defer log.InfoContext(ctx, "OS configuration loop exited.")
		if err := osConfigurator.runOSConfigurationLoop(ctx); err != nil {
			return trace.Wrap(err, "running OS configuration loop")
		}
		return errors.New("OS configuration loop terminated")
	})

	return g.Wait()
}

type embeddedApplicationServiceClient struct {
	service EmbeddedApplicationService

	mu         sync.Mutex
	userSigner crypto.Signer
	appSigners map[appKey]crypto.Signer
}

func (e *embeddedApplicationServiceClient) AuthenticateProcess(_ context.Context, req *vnetv1.AuthenticateProcessRequest, _ ...grpc.CallOption) (*vnetv1.AuthenticateProcessResponse, error) {
	return &vnetv1.AuthenticateProcessResponse{Version: req.GetVersion()}, nil
}

func (e *embeddedApplicationServiceClient) ReportNetworkStackInfo(context.Context, *vnetv1.ReportNetworkStackInfoRequest, ...grpc.CallOption) (*vnetv1.ReportNetworkStackInfoResponse, error) {
	return &vnetv1.ReportNetworkStackInfoResponse{}, nil
}

func (e *embeddedApplicationServiceClient) Ping(context.Context, *vnetv1.PingRequest, ...grpc.CallOption) (*vnetv1.PingResponse, error) {
	return &vnetv1.PingResponse{}, nil
}

func (e *embeddedApplicationServiceClient) ResolveFQDN(ctx context.Context, in *vnetv1.ResolveFQDNRequest, _ ...grpc.CallOption) (*vnetv1.ResolveFQDNResponse, error) {
	return e.service.ResolveFQDN(ctx, in)
}

func (e *embeddedApplicationServiceClient) ReissueAppCert(ctx context.Context, req *vnetv1.ReissueAppCertRequest, _ ...grpc.CallOption) (*vnetv1.ReissueAppCertResponse, error) {
	cert, err := e.service.GetAppCert(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appKey := newAppKey(
		req.GetAppInfo().GetAppKey(),
		uint16(req.GetTargetPort()),
	)

	e.mu.Lock()
	e.appSigners[appKey] = cert.PrivateKey.(crypto.Signer)
	e.mu.Unlock()

	return &vnetv1.ReissueAppCertResponse{Cert: cert.Certificate[0]}, nil
}

func (e *embeddedApplicationServiceClient) SignForApp(ctx context.Context, req *vnetv1.SignForAppRequest, _ ...grpc.CallOption) (*vnetv1.SignForAppResponse, error) {
	appKey := newAppKey(
		req.GetAppKey(),
		uint16(req.GetTargetPort()),
	)

	e.mu.Lock()
	signer, ok := e.appSigners[appKey]
	e.mu.Unlock()

	if !ok {
		return nil, trace.NotFound("no signer for app %v", appKey)
	}

	sig, err := sign(signer, req.GetSign())
	if err != nil {
		return nil, trace.Wrap(err, "signing for app %v", appKey)
	}
	return &vnetv1.SignForAppResponse{
		Signature: sig,
	}, nil
}

func (*embeddedApplicationServiceClient) OnNewAppConnection(context.Context, *vnetv1.OnNewAppConnectionRequest, ...grpc.CallOption) (*vnetv1.OnNewAppConnectionResponse, error) {
	return &vnetv1.OnNewAppConnectionResponse{}, nil
}

func (*embeddedApplicationServiceClient) OnInvalidLocalPort(context.Context, *vnetv1.OnInvalidLocalPortRequest, ...grpc.CallOption) (*vnetv1.OnInvalidLocalPortResponse, error) {
	return &vnetv1.OnInvalidLocalPortResponse{}, nil
}

func (e *embeddedApplicationServiceClient) GetTargetOSConfiguration(ctx context.Context, req *vnetv1.GetTargetOSConfigurationRequest, _ ...grpc.CallOption) (*vnetv1.GetTargetOSConfigurationResponse, error) {
	return e.service.GetTargetOSConfiguration(ctx, req)
}

func (e *embeddedApplicationServiceClient) UserTLSCert(ctx context.Context, req *vnetv1.UserTLSCertRequest, _ ...grpc.CallOption) (*vnetv1.UserTLSCertResponse, error) {
	cert, err := e.service.GetUserCert(ctx, req)
	if err != nil {
		return nil, err
	}

	// TODO: check how this works with multiple "profiles".
	e.mu.Lock()
	e.userSigner = cert.PrivateKey.(crypto.Signer)
	e.mu.Unlock()

	return &vnetv1.UserTLSCertResponse{
		Cert: cert.Certificate[0],
	}, nil
}

func (e *embeddedApplicationServiceClient) SignForUserTLS(ctx context.Context, req *vnetv1.SignForUserTLSRequest, _ ...grpc.CallOption) (*vnetv1.SignForUserTLSResponse, error) {
	e.mu.Lock()
	signer := e.userSigner
	e.mu.Unlock()

	if signer == nil {
		return nil, trace.NotFound("no user signer")
	}

	sig, err := sign(signer, req.GetSign())
	if err != nil {
		return nil, trace.Wrap(err, "signing for user")
	}
	return &vnetv1.SignForUserTLSResponse{
		Signature: sig,
	}, nil
}

func (*embeddedApplicationServiceClient) SessionSSHConfig(context.Context, *vnetv1.SessionSSHConfigRequest, ...grpc.CallOption) (*vnetv1.SessionSSHConfigResponse, error) {
	return nil, trace.NotImplemented("SSH not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) SignForSSHSession(context.Context, *vnetv1.SignForSSHSessionRequest, ...grpc.CallOption) (*vnetv1.SignForSSHSessionResponse, error) {
	return nil, trace.NotImplemented("SSH not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) ExchangeSSHKeys(context.Context, *vnetv1.ExchangeSSHKeysRequest, ...grpc.CallOption) (*vnetv1.ExchangeSSHKeysResponse, error) {
	// TODO: figure out what to do here instead (probably implement SSH support).
	//
	// SSH is not supported in embedded VNet, but we return a valid key here to
	// prevent it from blowing up completely.
	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := ssh.NewSignerFromSigner(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetv1.ExchangeSSHKeysResponse{
		UserPublicKey: signer.PublicKey().Marshal(),
	}, nil
}

var _ vnetv1.ClientApplicationServiceClient = (*embeddedApplicationServiceClient)(nil)
