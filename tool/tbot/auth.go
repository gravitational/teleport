package main

import (
	"crypto/x509"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// RegisterParams specifies parameters
// for first time register operation with auth server
type RegisterParams struct {
	// Token is a secure token to join the cluster
	Token string
	// ID is identity ID
	ID auth.IdentityID
	// Servers is a list of auth servers to dial
	Servers []utils.NetAddr
	// AdditionalPrincipals is a list of additional principals to dial
	AdditionalPrincipals []string
	// DNSNames is a list of DNS names to add to x509 certificate
	DNSNames []string
	// PrivateKey is a PEM encoded private key (not passed to auth servers)
	PrivateKey []byte
	// PublicTLSKey is a server's public key to sign
	PublicTLSKey []byte
	// PublicSSHKey is a server's public SSH key to sign
	PublicSSHKey []byte
	// CipherSuites is a list of cipher suites to use for TLS client connection
	CipherSuites []uint16
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string
	// CAPath is the path to the CA file.
	CAPath string
	// Clock specifies the time provider. Will be used to override the time anchor
	// for TLS certificate verification.
	// Defaults to real clock if unspecified
	Clock clockwork.Clock
}

// insecureRegisterClient attempts to connects to the Auth Server using the
// CA on disk. If no CA is found on disk, Teleport will not verify the Auth
// Server it is connecting to.
func insecureAuthClient(params RegisterParams) (*auth.Client, []*x509.Certificate, error) {
	var rootCAs []*x509.Certificate

	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now

	cert, err := readCA(params.CAPath)
	if err != nil && !trace.IsNotFound(err) {
		return nil, rootCAs, trace.Wrap(err)
	}

	// If no CA was found, then create a insecure connection to the Auth Server,
	// otherwise use the CA on disk to validate the Auth Server.
	if trace.IsNotFound(err) {
		tlsConfig.InsecureSkipVerify = true

		log.Warnf("Joining cluster without validating the identity of the Auth " +
			"Server. This may open you up to a Man-In-The-Middle (MITM) attack if an " +
			"attacker can gain privileged network access. To remedy this, use the CA pin " +
			"value provided when join token was generated to validate the identity of " +
			"the Auth Server.")
	} else {
		rootCAs = append(rootCAs, cert)

		certPool := x509.NewCertPool()
		certPool.AddCert(cert)
		tlsConfig.RootCAs = certPool

		log.Infof("Joining remote cluster %v, validating connection with certificate on disk.", cert.Subject.CommonName)
	}

	client, err := auth.NewClient(client.Config{
		Addrs: utils.NetAddrsToStrings(params.Servers),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		return nil, []*x509.Certificate{}, trace.Wrap(err)
	}

	return client, rootCAs, nil
}

// readCA will read in CA that will be used to validate the certificate that
// the Auth Server presents.
func readCA(path string) (*x509.Certificate, error) {
	certBytes, err := utils.ReadPath(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tlsca.ParseCertificatePEM(certBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate at %v", path)
	}
	return cert, nil
}

// pinAuthClient first connects to the Auth Server using a insecure
// connection to fetch the root CA. If the root CA matches the provided CA
// pin, a connection will be re-established and the root CA will be used to
// validate the certificate presented. If both conditions hold true, then we
// know we are connecting to the expected Auth Server.
func pinAuthClient(params RegisterParams) (*auth.Client, []*x509.Certificate, error) {
	// Build a insecure client to the Auth Server. This is safe because even if
	// an attacker were to MITM this connection the CA pin will not match below.
	tlsConfig := utils.TLSConfig(params.CipherSuites)
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.Time = params.Clock.Now
	authClient, err := auth.NewClient(client.Config{
		Addrs: utils.NetAddrsToStrings(params.Servers),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		return nil, []*x509.Certificate{}, trace.Wrap(err)
	}
	defer authClient.Close()

	// Fetch the root CA from the Auth Server. The NOP role has access to the
	// GetClusterCACert endpoint.
	localCA, err := authClient.GetClusterCACert()
	if err != nil {
		return nil, []*x509.Certificate{}, trace.Wrap(err)
	}
	certs, err := tlsca.ParseCertificatePEMs(localCA.TLSCA)
	if err != nil {
		return nil, []*x509.Certificate{}, trace.Wrap(err)
	}

	// Check that the SPKI pin matches the CA we fetched over a insecure
	// connection. This makes sure the CA fetched over a insecure connection is
	// in-fact the expected CA.
	err = utils.CheckSPKI(params.CAPins, certs)
	if err != nil {
		return nil, []*x509.Certificate{}, trace.Wrap(err)
	}

	for _, cert := range certs {
		// Check that the fetched CA is valid at the current time.
		err = utils.VerifyCertificateExpiry(cert, params.Clock)
		if err != nil {
			return nil, []*x509.Certificate{}, trace.Wrap(err)
		}

	}
	log.Infof("Connecting to remote cluster %v with CA pin.", certs[0].Subject.CommonName)

	// return these root CAs as we'll store them for later use
	var rootCAs []*x509.Certificate

	// Create another client, but this time with the CA provided to validate
	// that the Auth Server was issued a certificate by the same CA.
	tlsConfig = utils.TLSConfig(params.CipherSuites)
	tlsConfig.Time = params.Clock.Now
	certPool := x509.NewCertPool()
	for _, cert := range certs {
		certPool.AddCert(cert)
		rootCAs = append(rootCAs, cert)
	}
	tlsConfig.RootCAs = certPool

	authClient, err = auth.NewClient(client.Config{
		Addrs: utils.NetAddrsToStrings(params.Servers),
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
	})
	if err != nil {
		return nil, []*x509.Certificate{}, trace.Wrap(err)
	}

	return authClient, rootCAs, nil
}
