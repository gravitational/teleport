package pro

import (
	"cmp"
	"context"
	"crypto/tls"
	"encoding/pem"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/license"
	"github.com/gravitational/license/constants"
	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/api/cloud"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/licensefile"
	emodules "github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	//  defaultRequestTimeout is the default timeout of requests to fetch licenses
	defaultLicenseQueryTimeout = 60 * time.Second
	// defaultLicenseQueryInterval is how often Teleport will query Cloud for new licenses
	defaultLicenseQueryInterval = 5 * time.Minute
	// component is the name of the license updater component in logs
	component = "license_update"
	// licenseAPIEnvVarHostPort is an env var used to override the default cloud api server address
	licenseAPIEnvVarHostPort = "LICENSE_API_HOSTPORT"
)

// licenseUpdateServiceConfig is the license updater service config
type licenseUpdateServiceConfig struct {
	// ServerID is the server's ID
	ServerID string
	// LicenseFile is the license file reference
	LicenseFile *licensefile.LicenseFile
	// LicensePath is the path of the license on disk
	LicensePath string
	// Modules defines build time constraints and licensed features.
	Modules *emodules.EnterpriseModules
	// RequestTimeout is the timeout of the Cloud request
	RequestTimeout time.Duration
	// Interval is the interval Cloud should be queried for license updates.
	// If empty, defaults to a jittered value between 5 and 10 minutes.
	Interval time.Duration
	// NewClientFromTLSConfig is a factory function for creating a Cloud client
	// from a TLS configuration. Defaults to the production implementation but
	// can be replaced for testing purposes.
	NewClientFromTLSConfig func(*tls.Config) (cloud.Client, error)
	// Log is the logger
	Log *slog.Logger
}

func (c *licenseUpdateServiceConfig) CheckAndSetDefaults() error {
	if c.ServerID == "" {
		return trace.BadParameter("missing ServerID")
	}

	if c.LicenseFile == nil {
		return trace.BadParameter("missing LicenseFile")
	}

	if c.LicenseFile.KeyPair == nil {
		return trace.BadParameter("LicenseFile missing KeyPair")
	}

	if c.LicensePath == "" {
		return trace.BadParameter("missing LicensePath")
	}

	if c.Modules == nil {
		return trace.BadParameter("missing Modules")
	}

	if c.Interval <= 0 {
		c.Interval = retryutils.HalfJitter(defaultLicenseQueryInterval * 2)
	}

	if c.RequestTimeout <= 0 {
		c.RequestTimeout = defaultLicenseQueryTimeout
	}

	if c.Log == nil {
		c.Log = slog.With(teleport.ComponentKey, component)
	}

	if c.NewClientFromTLSConfig == nil {
		c.NewClientFromTLSConfig = newClientFromTLSConfig
	}

	return nil
}

// licenseUpdateService is a service that fetches Cloud for new
// licenses regularly, stores the new license and updates cluster features
// accordingly.
type licenseUpdateService struct {
	serverID               string
	licenseFile            *licensefile.LicenseFile
	licensePath            string
	requestTimeout         time.Duration
	interval               time.Duration
	client                 cloud.Client
	newClientFromTLSConfig func(*tls.Config) (cloud.Client, error)
	log                    *slog.Logger
	modules                *emodules.EnterpriseModules
}

// newLicenseUpdateService returns a new LicenseUpdateService
func newLicenseUpdateService(cfg licenseUpdateServiceConfig) (*licenseUpdateService, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := license.MakeTLSConfig(*cfg.LicenseFile.KeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cloudClient, err := cfg.NewClientFromTLSConfig(tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &licenseUpdateService{
		serverID:               cfg.ServerID,
		licenseFile:            cfg.LicenseFile,
		licensePath:            cfg.LicensePath,
		requestTimeout:         cfg.RequestTimeout,
		interval:               cfg.Interval,
		client:                 cloudClient,
		newClientFromTLSConfig: cfg.NewClientFromTLSConfig,
		log:                    cfg.Log,
		modules:                cfg.Modules,
	}, nil
}

// Run periodically queries and updates the license from Cloud's API
func (s *licenseUpdateService) Run(ctx context.Context) {
	s.log.InfoContext(ctx, "feature service started", "interval", s.interval)
	ticker := time.NewTicker(s.interval)

	defer ticker.Stop()
	defer s.client.Close()

	for {
		// fetch and update license immediately, to make sure the cluster will have updated
		// features shortly after startup.
		// This ensures that deployments where Teleport can't save the new license to disk won't run
		// for long with stale features until an updated license is received.
		if err := s.fetchAndUpdateLicense(ctx); err != nil {
			s.log.WarnContext(ctx, "error updating license", "error", err)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			s.log.DebugContext(ctx, "license update service has stopped")
			return
		}
	}
}

// fetchAndUpdateLicense updates the modules features and the Cloud client, and stores the new license on disk.
func (s *licenseUpdateService) fetchAndUpdateLicense(ctx context.Context) error {
	s.log.InfoContext(ctx, "fetching updated license")
	reqCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()

	resp, err := s.client.GetUpdatedLicense(reqCtx, &v1.GetUpdatedLicenseRequest{
		AuthVersion: "v" + teleport.Version,
		ServerId:    s.serverID,
	})
	if err != nil {
		if code := status.Code(err); code == codes.Unavailable || code == codes.DeadlineExceeded {
			return trace.ConnectionProblem(err, "could not fetch updated license, please ensure that Teleport has connectivity to %v", s.client.Hostname())
		}
		return trace.Wrap(err, "error fetching updated license")
	}

	// If server sends an empty response it means the license didn't change, so
	// no additional work is necessary.
	if resp == nil || resp.Pem == "" {
		s.log.InfoContext(ctx, "no updates to license")
		return nil
	}

	// The license returned by the server doesn't include the anonymization key,
	// which is generated when the license is download from the Dashboard, so
	// we append its block to the Server's response PEM
	newPem, err := appendAnonymizationKey([]byte(resp.Pem), []byte(s.licenseFile.License.GetAnonymizationKey()))
	if err != nil {
		return trace.Wrap(err, "error appending anonymization key to new license")
	}
	newLicense, err := licensefile.FromPEM(newPem)
	if err != nil {
		return trace.Wrap(err, "error converting received PEM to license")
	}

	// set features
	s.licenseFile = newLicense

	// TODO(tross): create the client a single time instead of in LoadFeatures
	// and updateClientLicense and resuse it.
	features, err := LoadFeatures(ctx, newLicense)
	if err != nil {
		return trace.Wrap(err, "error loading features from new license")
	}

	s.modules.UpdateModules(newLicense, features)

	s.log.InfoContext(ctx, "license updated",
		"features", modules.GetModules().Features(),
		"not_before", newLicense.KeyPair.Cert.NotBefore,
		"not_after", newLicense.KeyPair.Cert.NotAfter,
	)

	// Update the Cloud client so it has the updated license file so the server
	// sees that we have the new license and don't send it again.
	// The old license is still valid for authorization purposes, so other
	// services using the Cloud API don't need to create a new client.
	if err := s.updateClientLicense(ctx); err != nil {
		// It is OK to proceed if updating the client failed.
		// The server will send the license again in the next request since the
		// client will present the old license again.
		s.log.WarnContext(ctx, "error resetting cloud Client", "error", err)
	}

	// save new license on disk
	err = writeLicense(newPem, s.licensePath)
	if err != nil {
		// The Teleport process may lack permission to write to the license, or be running on an environment
		// where it can't write to disk, like a k8s cluster.
		// Since we already updated the in-memory license and the modules, the server will enable the new features.
		return trace.Wrap(err, "new license was downloaded but couldn't be updated on disk. Cluster features were updated successfully.")
	}
	return nil
}

// LoadFeatures consumes the license to determine which features are enabled. If
// the process is running in a Cloud environment the features will be loaded via
// an API call to Cloud.
func LoadFeatures(ctx context.Context, licenseFile *licensefile.LicenseFile) (modules.Features, error) {
	if licenseFile == nil || licenseFile.License == nil {
		return modules.Features{}, nil
	}

	if !licenseFile.License.GetCloud() {
		return emodules.GetSelfHostedLicenseFeatures(licenseFile.License), nil
	}

	slog.DebugContext(ctx, "fetching features from Cloud")
	tlsConfig, err := license.MakeTLSConfig(*licenseFile.KeyPair)
	if err != nil {
		return modules.Features{}, trace.Wrap(err)
	}

	client, err := cloud.NewClientFromTLSConfig(tlsConfig)
	if err != nil {
		slog.ErrorContext(ctx, "failed creating cloud client to fetch features", "error", err)
		return modules.Features{}, trace.Wrap(err)
	}
	defer client.Close()

	const cloudFeatureRequestTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(ctx, cloudFeatureRequestTimeout)
	defer cancel()

	f, err := feature.GetCloudFeatures(ctx, client)
	if err != nil {
		slog.ErrorContext(ctx, "failed fetching features from Cloud", "error", err)
		return modules.Features{}, trace.Wrap(err)
	}

	slog.DebugContext(ctx, "successfully fetched features from Cloud", "features", f.ToProto())
	return *f, nil
}

// updateClientLicense refreshes the Cloud client by creating a new one with
// the updated mTLS certificate from the latest license. This ensures that the
// client uses the new license for secure communication.
// The current client is closed before replacing it with the new client.
func (s *licenseUpdateService) updateClientLicense(ctx context.Context) error {
	tlsConfig, err := license.MakeTLSConfig(*s.licenseFile.KeyPair)
	if err != nil {
		return trace.Wrap(err)
	}
	newClient, err := s.newClientFromTLSConfig(tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.client.Close(); err != nil {
		// log the error and proceed to use the newly created client
		s.log.ErrorContext(ctx, "error closing Cloud client", "error", err)
	}
	s.client = newClient
	return nil
}

// writeLicense atomically writes PEM into licensePath, preserving its permission flags.
func writeLicense(pem []byte, licensePath string) error {
	dir := filepath.Dir(licensePath)
	prevLicenseInfo, err := os.Stat(licensePath)
	if err != nil {
		return trace.Wrap(err, "failed to locate existing license file at %s", licensePath)
	}
	perms := prevLicenseInfo.Mode().Perm()

	// create temp file
	tempFile, err := os.CreateTemp(dir, "temp-license-*")
	if err != nil {
		return trace.Wrap(err, "error creating temporary license file")
	}

	defer func() {
		if tempFile != nil {
			//nolint:errcheck // another error is being returned; ignore these
			tempFile.Close()
			os.Remove(tempFile.Name())
		}
	}()

	// preserver license's permissions
	if err := tempFile.Chmod(perms); err != nil {
		return trace.Wrap(err, "could not change tempfile permissions")
	}

	// write license to temp file
	if _, err := tempFile.Write(pem); err != nil {
		return trace.Wrap(err, "error writing new license to temporary file")
	}
	if err := tempFile.Sync(); err != nil {
		return trace.Wrap(err, "error flushing temporary license file")
	}
	if err := tempFile.Close(); err != nil {
		return trace.Wrap(err, "error closing temporary license file")
	}

	// swap files, os.Rename is atomic on Unix systems
	err = os.Rename(tempFile.Name(), licensePath)
	if err != nil {
		return trace.Wrap(err)
	}
	// set tempLicense to nil to avoid trying to close
	// and remove it in the defer func
	tempFile = nil

	return nil
}

// appendAnonymizationKey appends a PEM-encoded annonymization block to the end of the license
func appendAnonymizationKey(licensePEM []byte, anonKey []byte) ([]byte, error) {
	newBlock := &pem.Block{
		Type: constants.AnonymizationKeyPEMBlock,
		Headers: map[string]string{
			"Purpose": "Anonymization of Teleport user activity and resource usage statistics",
			"Caution": "Please ensure that this key is the same in all Teleport instances",
		},
		Bytes: anonKey,
	}

	return append(licensePEM, pem.EncodeToMemory(newBlock)...), nil
}

// newClientFromTLSConfig creates a cloud client using the provided tls.Config.
// The cloud server address defaults to the production Cloud address (api.teleport.sh), but
// can be overridden via the LICENSE_API_HOSTPORT env var.
func newClientFromTLSConfig(cfg *tls.Config) (cloud.Client, error) {
	cloudAPIServerAddr := cmp.Or(os.Getenv(licenseAPIEnvVarHostPort), cloud.DefaultAPIServerAddr)

	apiServerAddr, err := utils.ParseHostPortAddr(cloudAPIServerAddr, cloud.DefaultAPIServerPort)
	if err != nil {
		return nil, trace.BadParameter("error parsing cloud client address: %s", err.Error())
	}

	if cfg == nil {
		return nil, trace.BadParameter("nil tls config")
	}

	tlsCfg := cfg.Clone()
	tlsCfg.ServerName = apiServerAddr.Host()
	tlsCfg.InsecureSkipVerify = lib.IsInsecureDevMode()

	return cloud.NewClient(cloud.ClientConfig{
		Hostname:  apiServerAddr.Addr,
		TLSConfig: tlsCfg,
	})
}
