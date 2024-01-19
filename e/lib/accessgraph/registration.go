package accessgraph

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

// Registrator is a thin layer around methods of the same name in the accessgraphv1alpha service.
// Its purpose is to help decouple the logic from concrete transport (gRPC).
type Registrator interface {
	Register(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, clusterName string) error
	ReplaceCAs(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error
}

// registrator is a concrete implementation of Registrator
type registrator struct{}

// Register implements Registrator.
func (*registrator) Register(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, hostCAPem []byte, clusterName string) error {
	conn, err := NewAccessGraphClient(ctx, config, creds)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	client := accessgraphv1.NewAccessGraphServiceClient(conn)

	_, err = client.Register(ctx, &accessgraphv1.RegisterRequest{
		HostCaPem:   hostCAPem,
		ClusterName: clusterName,
	})
	return trace.Wrap(err)
}

// ReplaceCAs implements Registrator.
func (*registrator) ReplaceCAs(ctx context.Context, config ServiceClientConfig, creds ClientCredentials, caPEMs [][]byte) error {
	conn, err := NewAccessGraphClient(ctx, config, creds)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()
	client := accessgraphv1.NewAccessGraphServiceClient(conn)

	_, err = client.ReplaceCAs(ctx, &accessgraphv1.ReplaceCAsRequest{HostCaPem: caPEMs})
	return trace.Wrap(err)
}

type authServer interface {
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// Register registers the cluster as a tenant with the Access Graph server,
// and submits additional Host CA certificates (if any exist due to ongoing CA rotation).
func Register(ctx context.Context, log *logrus.Entry, reg Registrator, config ServiceClientConfig, adminCreds ClientCredentials, auth authServer, license *licensefile.LicenseFile) error {
	clusterName, err := auth.GetClusterName()
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
	var activeCA = activeKeys.TLS[0].Cert
	var additionalCA []byte
	caToRegister := activeCA
	if hostCA.GetRotation().Phase == types.RotationPhaseUpdateClients {
		// In the UpdateClients phase, we're starting up with the new CA as "active",
		// but since the new CA has not yet been submitted to TAG,
		// we must register using the old CA to be identified as the existing tenant
		additionalKeys := hostCA.GetAdditionalTrustedKeys()
		if len(additionalKeys.TLS) == 0 {
			return trace.BadParameter("expected the additional CA keypair to exist")
		}
		additionalCA = additionalKeys.TLS[0].Cert
		caToRegister = additionalCA
	}

	// credentials used to invoke Register()
	regCreds := adminCreds
	// In cloud, registration CA is the licensing CA.
	if modules.GetModules().Features().Cloud {
		regCreds = ClientCredentials{
			CertPEM: license.KeyPair.CertPEM,
			KeyPEM:  license.KeyPair.KeyPEM,
		}
	}

	// It is possible for Register() to fail despite the fact we have previously registered successfully.
	// For example, in self-hosted environments after CA rotation we are authenticating Register()
	// with a certificate signed by the new Host CA,
	// but the user might not have updated `registration_cas` in the TAG service config.
	// We should only raise this as a hard error if subsequent ReplaceCAs() call fails,
	// as it indicates that authentication with this Host CA fails in general (i.e. the Host CA is not trusted).
	regErr := reg.Register(ctx, config, regCreds, caToRegister, clusterName.GetClusterName())

	cas := [][]byte{activeCA}
	if additionalCA != nil {
		cas = append(cas, additionalCA)
	}

	err = reg.ReplaceCAs(ctx, config, adminCreds, cas)
	if err != nil {
		return trace.NewAggregate(regErr, err)
	}

	return nil
}
