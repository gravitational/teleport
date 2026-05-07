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

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

// EmbeddedVNetConfig is the configuration for an embedded/single-process
// application of VNet.
type EmbeddedVNetConfig struct {
	// Device is the TUN device VNet will bind to.
	Device TUNDevice

	// ApplicationService is used to provide configuration, resolve DNS queries,
	// and generate client certificates.
	ApplicationService EmbeddedApplicationService

	// ConfigureHost is called to update the host's routing table and DNS
	// resolver configuration. It will be called multiple times, so must be
	// idempotent. cfg will be nil when VNet it shutting down and any host
	// configuration should be torn down.
	ConfigureHost EmbeddedConfigureHostFunc

	// UpstreamNameserverSource overrides the default platform source of upstream
	// nameservers (e.g. allowing you to swap systemd-resolved for /etc/resolv.conf
	// on Linux).
	UpstreamNameserverSource dns.UpstreamNameserverSource
}

// EmbeddedConfigureHostFunc is called to update the host's routing table and
// DNS resolver configuration.
type EmbeddedConfigureHostFunc func(ctx context.Context, cfg *EmbeddedVNetHostConfig) error

// EmbeddedApplicationService is a subset of vnetv1.ClientApplicationServiceClient
// required for embedded/single-process applications of VNet.
type EmbeddedApplicationService interface {
	// ResolveFQDN is called during DNS resolution to resolve a fully-qualified
	// domain name to a target.
	ResolveFQDN(ctx context.Context, fqdn string) (*vnetv1.ResolveFQDNResponse, error)

	// GetTargetOSConfiguration gets the target OS configuration.
	GetTargetOSConfiguration(ctx context.Context) (*vnetv1.TargetOSConfiguration, error)

	// GetAppCert issues a TLS certificate for the given application.
	GetAppCert(ctx context.Context, key *vnetv1.AppInfo, port uint16) (*tls.Certificate, error)

	// GetAppSigner returns the private key for the given application's TLS certificate.
	GetAppSigner(ctx context.Context, key *vnetv1.AppKey, port uint16) (crypto.Signer, error)
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

	// DNSAddrs are the IPv4 and IPv6 addresses for the VNet's nameserver.
	DNSAddrs []string

	// DNSZones are the zones for which the host's DNS resolver should send
	// queries to the VNet's nameserver.
	DNSZones []string
}

// EmbeddedVNet is an embedded/single-process application of VNet. It does not
// currently support VNet SSH.
type EmbeddedVNet struct {
	device              TUNDevice
	client              *clientApplicationServiceClient
	configureHost       EmbeddedConfigureHostFunc
	upstreamNameservers dns.UpstreamNameserverSource
}

// NewEmbeddedVNet creates an embedded VNet with the given configuration.
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
				service: cfg.ApplicationService,
			},
			closer: io.NopCloser(bytes.NewReader([]byte{})),
		},
		configureHost:       cfg.ConfigureHost,
		upstreamNameservers: cfg.UpstreamNameserverSource,
	}, nil
}

// Run the VNet until the given context is canceled.
func (vnet *EmbeddedVNet) Run(ctx context.Context) error {
	stackConfig, err := newNetworkStackConfig(ctx, vnet.device, vnet.client)
	if err != nil {
		return trace.Wrap(err, "creating network stack config")
	}
	if vnet.upstreamNameservers != nil {
		stackConfig.upstreamNameserverSource = vnet.upstreamNameservers
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
			if oc.tunName == "" {
				return vnet.configureHost(ctx, nil)
			}
			return vnet.configureHost(ctx, &EmbeddedVNetHostConfig{
				DeviceIPv4: oc.tunIPv4,
				DeviceIPv6: oc.tunIPv6,
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

	err = g.Wait()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

type embeddedApplicationServiceClient struct{ service EmbeddedApplicationService }

func (e *embeddedApplicationServiceClient) AuthenticateProcess(_ context.Context, req *vnetv1.AuthenticateProcessRequest, _ ...grpc.CallOption) (*vnetv1.AuthenticateProcessResponse, error) {
	return &vnetv1.AuthenticateProcessResponse{Version: req.GetVersion()}, nil
}

func (e *embeddedApplicationServiceClient) ReportNetworkStackInfo(context.Context, *vnetv1.ReportNetworkStackInfoRequest, ...grpc.CallOption) (*vnetv1.ReportNetworkStackInfoResponse, error) {
	return &vnetv1.ReportNetworkStackInfoResponse{}, nil
}

func (e *embeddedApplicationServiceClient) Ping(context.Context, *vnetv1.PingRequest, ...grpc.CallOption) (*vnetv1.PingResponse, error) {
	return &vnetv1.PingResponse{}, nil
}

func (e *embeddedApplicationServiceClient) ResolveFQDN(ctx context.Context, req *vnetv1.ResolveFQDNRequest, _ ...grpc.CallOption) (*vnetv1.ResolveFQDNResponse, error) {
	return e.service.ResolveFQDN(ctx, req.GetFqdn())
}

func (e *embeddedApplicationServiceClient) ReissueAppCert(ctx context.Context, req *vnetv1.ReissueAppCertRequest, _ ...grpc.CallOption) (*vnetv1.ReissueAppCertResponse, error) {
	cert, err := e.service.GetAppCert(ctx, req.GetAppInfo(), uint16(req.GetTargetPort()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetv1.ReissueAppCertResponse{Cert: cert.Certificate[0]}, nil
}

func (e *embeddedApplicationServiceClient) SignForApp(ctx context.Context, req *vnetv1.SignForAppRequest, _ ...grpc.CallOption) (*vnetv1.SignForAppResponse, error) {
	signer, err := e.service.GetAppSigner(ctx, req.GetAppKey(), uint16(req.GetTargetPort()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sig, err := sign(signer, req.GetSign())
	if err != nil {
		return nil, trace.Wrap(err, "signing for app %v", req.GetAppKey())
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

func (e *embeddedApplicationServiceClient) GetTargetOSConfiguration(ctx context.Context, _ *vnetv1.GetTargetOSConfigurationRequest, _ ...grpc.CallOption) (*vnetv1.GetTargetOSConfigurationResponse, error) {
	cfg, err := e.service.GetTargetOSConfiguration(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetv1.GetTargetOSConfigurationResponse{TargetOsConfiguration: cfg}, nil
}

func (e *embeddedApplicationServiceClient) UserTLSCert(ctx context.Context, req *vnetv1.UserTLSCertRequest, _ ...grpc.CallOption) (*vnetv1.UserTLSCertResponse, error) {
	return nil, trace.NotImplemented("SSH not supported in embedded VNet")
}

func (e *embeddedApplicationServiceClient) SignForUserTLS(context.Context, *vnetv1.SignForUserTLSRequest, ...grpc.CallOption) (*vnetv1.SignForUserTLSResponse, error) {
	return nil, trace.NotImplemented("SSH not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) SessionSSHConfig(context.Context, *vnetv1.SessionSSHConfigRequest, ...grpc.CallOption) (*vnetv1.SessionSSHConfigResponse, error) {
	return nil, trace.NotImplemented("SSH not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) SignForSSHSession(context.Context, *vnetv1.SignForSSHSessionRequest, ...grpc.CallOption) (*vnetv1.SignForSSHSessionResponse, error) {
	return nil, trace.NotImplemented("SSH not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) ReissueDBCert(ctx context.Context, in *vnetv1.ReissueDBCertRequest, opts ...grpc.CallOption) (*vnetv1.ReissueDBCertResponse, error) {
	return nil, trace.NotImplemented("databases not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) SignForDB(context.Context, *vnetv1.SignForDBRequest, ...grpc.CallOption) (*vnetv1.SignForDBResponse, error) {
	return nil, trace.NotImplemented("databases not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) OnNewDBConnection(context.Context, *vnetv1.OnNewDBConnectionRequest, ...grpc.CallOption) (*vnetv1.OnNewDBConnectionResponse, error) {
	return nil, trace.NotImplemented("databases not supported in embedded VNet")
}

func (*embeddedApplicationServiceClient) ExchangeSSHKeys(context.Context, *vnetv1.ExchangeSSHKeysRequest, ...grpc.CallOption) (*vnetv1.ExchangeSSHKeysResponse, error) {
	// EmbeddedVNet does not yet support SSH, but we must return a valid public
	// key so that the SSH setup code doesn't return an error.
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

func (*embeddedApplicationServiceClient) PerformSessionMFACeremony(context.Context, *vnetv1.PerformSessionMFACeremonyRequest, ...grpc.CallOption) (*vnetv1.PerformSessionMFACeremonyResponse, error) {
	return nil, trace.NotImplemented("MFA ceremonies not supported in embedded VNet")
}

var _ vnetv1.ClientApplicationServiceClient = (*embeddedApplicationServiceClient)(nil)
