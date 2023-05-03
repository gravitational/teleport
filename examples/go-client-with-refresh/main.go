package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	teleUtils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"sync"
	"time"
)

type dynamicIdentityFile struct {
	mu      sync.RWMutex
	tlsCert *tls.Certificate
	pool    *x509.CertPool

	log         logrus.FieldLogger
	clusterName string
}

func NewDynamicIdentityFile(path string) (*dyanamicIdentityFile, error) {

	return &dyanamicIdentityFile{}, nil
}

func (d *dynamicIdentityFile) LoadFromIdentityFile(id *identityfile.IdentityFile) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	cert, err := keys.X509KeyPair(id.Certs.TLS, id.PrivateKey)
	if err != nil {
		return trace.Wrap(err)
	}
	d.tlsCert = &cert

	pool := x509.NewCertPool()
	for _, caCerts := range id.CACerts.TLS {
		if !pool.AppendCertsFromPEM(caCerts) {
			return trace.BadParameter("invalid CA cert PEM")
		}
	}
	d.pool = pool
	return nil
}

func (d *dynamicIdentityFile) Dialer(cfg client.Config) (client.ContextDialer, error) {
	// Returning a dialer isn't necessary for this credential.
	return nil, trace.NotImplemented("no dialer")
}

func (d *dynamicIdentityFile) TLSConfig() (*tls.Config, error) {
	cfg := &tls.Config{
		// GetClientCertificate is used instead of the static Certificates
		// field.
		Certificates: nil,
		// Encoded cluster name required to ensure requests are routed to the
		// correct cloud tenants.
		ServerName: utils.EncodeClusterName(d.clusterName),
		GetClientCertificate: func(info *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			// We
			d.log.Debug("GetClientCertificate called")
			d.mu.RLock()
			defer d.mu.RUnlock()
			return d.tlsCert, nil
		},
		// InsecureSkipVerify is forced true to ensure that only our
		// VerifyConnection callback is used to verify the server's presented
		// certificate.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			// This VerifyConnection callback is based on the standard library
			// implementation of verifyServerCertificate in the tls package.
			// We provide our own implementation so we can dynamically handle
			// a changing CA Roots pool.
			d.log.WithFields(logrus.Fields{
				"negotiated_protocol": state.NegotiatedProtocol,
				"server_name":         state.ServerName,
			}).Debug("VerifyConnection called")
			d.mu.RLock()
			rootPool := d.pool.Clone()
			d.mu.RUnlock()

			opts := x509.VerifyOptions{
				DNSName:       state.ServerName,
				Intermediates: x509.NewCertPool(),
				Roots:         rootPool,
			}
			for _, cert := range state.PeerCertificates[1:] {
				// Whilst we don't currently use intermediate certs at
				// Teleport, including this here means that we are
				// future-proofed in case we do.
				opts.Intermediates.AddCert(cert)
			}
			_, err := state.PeerCertificates[0].Verify(opts)
			return err
		},
		NextProtos: []string{http2.NextProtoTLS},
	}

	return cfg, nil
}

func (d *dyanamicIdentityFile) SSHClientConfig() (*ssh.ClientConfig, error) {
	// For now, SSH Client Config is disabled until I can wrap my head around
	// the complexities introduced by TLS over SSH.
	// This means the auth server must be available directly or using
	// the ALPN/SNI.
	return nil, trace.NotImplemented("no ssh config")
}

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		unix.SIGTERM,
		unix.SIGINT,
	)
	defer cancel()

	log := teleUtils.NewLogger()
	if err := run(ctx, log); err != nil {
		log.Fatal(err)
	}
}

const dynamicCred = true

func run(ctx context.Context, log logrus.FieldLogger) error {
	proxyAddr := os.Getenv("PROXY_ADDR")
	identityFilePath := os.Getenv("TELEPORT_IDENTITY_FILE")
	clusterName := os.Getenv("CLUSTER_NAME")

	var credential client.Credentials
	if dynamicCred == true {
		cred := &dyanamicIdentityFile{
			log:         log,
			clusterName: clusterName,
		}
		idFile, err := identityfile.ReadFile(identityFilePath)
		if err != nil {
			return trace.Wrap(err, "reading identity file")
		}
		if err := cred.LoadFromIdentityFile(idFile); err != nil {
			return trace.Wrap(err, "loading identity file")
		}
		go func() {
			// This goroutine loop could be replaced with a file watcher.
			for {
				time.Sleep(time.Second * 30)
				idFile, err := identityfile.ReadFile(identityFilePath)
				if err != nil {
					log.WithError(err).Warn("Failed to re-read identity file")
					continue
				}
				if err := cred.LoadFromIdentityFile(idFile); err != nil {
					log.WithError(err).Warn("Failed to re-load identity file")
					continue
				}
				log.Info("Succeeded in re-reading and re-loading identity file from disk. New client connections will use this identity.")
			}
		}()
		credential = cred
	} else {
		credential = client.LoadIdentityFile(identityFilePath)
		credential = client.LoadProfile("", "")
	}

	cfg := client.Config{
		Addrs: []string{proxyAddr},
		Credentials: []client.Credentials{
			credential,
		},
		DialOpts: []grpc.DialOption{
			// grpc.WithReturnConnectionError(),
		},
		ALPNSNIAuthDialClusterName: clusterName,
		DialInBackground:           true,
	}
	clt, err := client.New(ctx, cfg)
	if err != nil {
		return trace.Wrap(err, "creating client")
	}
	defer clt.Close()

	return monitorLoop(ctx, log, clt)
}

func monitorLoop(
	ctx context.Context,
	log logrus.FieldLogger,
	clt *client.Client,
) error {
	for {
		// Exit is context is cancelled.
		if err := ctx.Err(); err != nil {
			log.Info(
				"Detected context cancellation, exiting watch loop!",
			)
			return nil
		}

		// This action represents any unary action against the Teleport API
		start := time.Now()
		nodes, err := clt.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			log.WithError(err).Error("failed to fetch nodes")
		} else {
			log.WithFields(logrus.Fields{
				"count":    len(nodes),
				"duration": time.Since(start),
			}).Info("Fetched nodes list")
		}

		time.Sleep(5 * time.Second)
	}
}
