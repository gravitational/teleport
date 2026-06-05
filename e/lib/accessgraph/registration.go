package accessgraph

import (
	"context"
	"crypto/tls"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/modules"
)

// Registrator is a thin layer around methods of the same name in the accessgraphv1alpha service.
// Its purpose is to help decouple the logic from concrete transport (gRPC).
type Registrator interface {
	Register(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPems [][]byte, clusterName string) error
	ReplaceCAs(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error
}

// registrator is a concrete implementation of Registrator
type registrator struct{}

// Register implements Registrator.
func (*registrator) Register(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, hostCAPems [][]byte, clusterName string) error {
	conn, err := NewAccessGraphClient(ctx, config, getCreds)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	client := accessgraphv1.NewAccessGraphServiceClient(conn)
	if len(hostCAPems) == 0 {
		return trace.BadParameter("expected at least one Host CA PEM to register")
	}
	_, err = client.Register(ctx, accessgraphv1.RegisterRequest_builder{
		// Keep compatibility with older TAG versions that expect a single Host CA PEM.
		HostCaPem:   hostCAPems[0],
		HostCaPems:  hostCAPems,
		ClusterName: clusterName,
	}.Build())
	return trace.Wrap(err)
}

// ReplaceCAs implements Registrator.
func (*registrator) ReplaceCAs(ctx context.Context, config ServiceClientConfig, getCreds ClientCredentialsGetter, caPEMs [][]byte) error {
	conn, err := NewAccessGraphClient(ctx, config, getCreds)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	client := accessgraphv1.NewAccessGraphServiceClient(conn)

	_, err = client.ReplaceCAs(ctx, accessgraphv1.ReplaceCAsRequest_builder{HostCaPem: caPEMs}.Build())
	return trace.Wrap(err)
}

type authServer interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GetClusterName(ctx context.Context) (types.ClusterName, error)
}

// Register registers the cluster as a tenant with the Access Graph server,
// and submits additional Host CA certificates (if any exist due to ongoing CA rotation).
func Register(ctx context.Context, reg Registrator, config ServiceClientConfig, getAdminCreds ClientCredentialsGetter, auth authServer, license *licensefile.LicenseFile, mod modules.Modules) error {
	// we need to call Register and ReplaceCAs with the same identity if we're
	// not using the license file as registration credentials
	adminCreds, err := getAdminCreds()
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName, err := auth.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	hostCA, err := auth.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.HostCA,
		DomainName: clusterName.GetClusterName(),
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}

	activeKeys := hostCA.GetActiveKeys()
	if len(activeKeys.TLS) == 0 {
		return trace.BadParameter("expected the active keyset to not be empty")
	}
	var casToRegister [][]byte
	for _, keyPair := range activeKeys.TLS {
		casToRegister = append(casToRegister, keyPair.Cert)
	}

	if hostCA.GetRotation().Phase == types.RotationPhaseUpdateClients {
		// In the UpdateClients phase, we're starting up with the new CA as "active",
		// but since the new CA has not yet been submitted to TAG,
		// we must register using the old CA to be identified as the existing tenant
		additionalKeys := hostCA.GetAdditionalTrustedKeys()
		if len(additionalKeys.TLS) == 0 {
			return trace.BadParameter("expected the additional CA keypair to exist")
		}
		for _, keyPair := range additionalKeys.TLS {
			casToRegister = append(casToRegister, keyPair.Cert)
		}
	}

	// credentials used to invoke Register()
	regCreds := adminCreds
	// In cloud, registration CA is the licensing CA.
	if mod.Features().Cloud {
		c, err := tls.X509KeyPair(license.KeyPair.CertPEM, license.KeyPair.KeyPEM)
		if err != nil {
			return trace.Wrap(err)
		}
		regCreds = &c
	}

	// It is possible for Register() to fail despite the fact we have previously registered successfully.
	// For example, in self-hosted environments after CA rotation we are authenticating Register()
	// with a certificate signed by the new Host CA,
	// but the user might not have updated `registration_cas` in the TAG service config.
	// We should only raise this as a hard error if subsequent ReplaceCAs() call fails,
	// as it indicates that authentication with this Host CA fails in general (i.e. the Host CA is not trusted).
	regErr := reg.Register(ctx, config, func() (*tls.Certificate, error) { return regCreds, nil }, casToRegister, clusterName.GetClusterName())

	err = reg.ReplaceCAs(ctx, config, func() (*tls.Certificate, error) { return adminCreds, nil }, casToRegister)
	if err != nil {
		return trace.NewAggregate(regErr, err)
	}

	return nil
}
