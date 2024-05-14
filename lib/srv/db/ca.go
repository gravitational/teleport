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

package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/tlsca"
)

// startCARenewer renewer which is going to renew cloud-based database CA.
func (s *Server) startCARenewer(ctx context.Context) {
	schedule := s.cfg.Clock.NewTicker(caRenewInterval)
	defer schedule.Stop()

	for {
		select {
		case <-schedule.Chan():
			for _, database := range s.getProxiedDatabases() {
				if err := s.initCACert(ctx, database); err != nil {
					s.log.WithError(err).Errorf("Failed to renew database %q CA.", database.GetName())
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// initCACert initializes the provided server's CA certificate in case of a
// cloud hosted database instance.
func (s *Server) initCACert(ctx context.Context, database types.Database) error {
	s.mu.RLock()
	if !s.shouldInitCACertLocked(database) {
		s.mu.RUnlock()
		return nil
	}
	// make a copy so we can safely unlock before doing expensive CA cert
	// version checking or downloading.
	copy := database.Copy()
	s.mu.RUnlock()
	bytes, err := s.getCACerts(ctx, copy)
	if err != nil {
		return trace.Wrap(err)
	}
	// Make sure the cert we got is valid just in case.
	if _, err := tlsca.ParseCertificatePEM(bytes); err != nil {
		return trace.Wrap(err, "CA certificate for %v doesn't appear to be a valid x509 certificate: %s",
			copy, bytes)
	}
	s.mu.Lock()
	// update the original database under a lock, since we're mutating it.
	database.SetStatusCA(string(bytes))
	s.mu.Unlock()
	return nil
}

// shouldInitCACertLocked returns whether a given database needs to have its
// CA cert initialized.
// The caller must call RLock on `s.mu` before calling this function.
func (s *Server) shouldInitCACertLocked(database types.Database) bool {
	// To identify if the CA cert was set automatically, compare the result of
	// `GetCA` (which can return user-provided CA) with `GetStatusCA`, which
	// only returns the CA set by the Teleport. If both contents differ, we will
	// not download CAs for the database. Both sides will be empty at the first
	// pass, downloading and populating the `StatusCA`.
	if database.GetCA() != database.GetStatusCA() {
		return false
	}
	// Can only download it for cloud-hosted instances.
	switch database.GetType() {
	case types.DatabaseTypeRDS,
		types.DatabaseTypeRedshift,
		types.DatabaseTypeRedshiftServerless,
		types.DatabaseTypeElastiCache,
		types.DatabaseTypeMemoryDB,
		types.DatabaseTypeAWSKeyspaces,
		types.DatabaseTypeDynamoDB,
		types.DatabaseTypeMongoAtlas,
		types.DatabaseTypeCloudSQL,
		types.DatabaseTypeAzure:
		return true
	default:
		return false
	}
}

// getCACerts updates and returns automatically downloaded root certificate for
// the provided cloud database instance.
func (s *Server) getCACerts(ctx context.Context, database types.Database) ([]byte, error) {
	// Auto-downloaded certs reside in the data directory.
	filePaths, err := s.getCACertPaths(database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var all [][]byte
	for _, filePath := range filePaths {
		caBytes, err := s.getCACert(ctx, database, filePath)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		all = append(all, caBytes)
	}

	// Add new lines between files in case one doesn't end with a new line.
	// It's ok if there are multiple new lines between certs.
	return bytes.Join(all, []byte("\n")), nil
}

// getCACert returns the downloaded certificate for provided database and file
// path.
//
// The cert is going to be updated and persisted into the filesystem.
func (s *Server) getCACert(ctx context.Context, database types.Database, filePath string) ([]byte, error) {
	// Try to update the certificate.
	err := s.updateCACert(ctx, database, filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// The update flow is going to create/update the cached CA, so we can read
	// the contents from it.
	s.log.Debugf("Loaded CA certificate %v.", filePath)
	return os.ReadFile(filePath)
}

// getCACertPaths returns the paths where automatically downloaded root certificate
// for the provided database is stored in the filesystem.
func (s *Server) getCACertPaths(database types.Database) ([]string, error) {
	switch database.GetType() {
	// All RDS instances share the same root CA (per AWS region) which can be
	// downloaded from a well-known URL.
	case types.DatabaseTypeRDS:
		return []string{filepath.Join(s.cfg.DataDir, filepath.Base(rdsCAURLForDatabase(database)))}, nil

	// All Redshift instances share the same root CA which can be downloaded
	// from a well-known URL.
	//
	// https://docs.aws.amazon.com/redshift/latest/mgmt/connecting-ssl-support.html
	case types.DatabaseTypeRedshift,
		types.DatabaseTypeRedshiftServerless:
		return []string{filepath.Join(s.cfg.DataDir, filepath.Base(redshiftCAURLForDatabase(database)))}, nil

	// ElastiCache databases are signed with Amazon root CA. In most cases,
	// x509.SystemCertPool should be sufficient to verify ElastiCache servers.
	// However, x509.SystemCertPool does not support windows for go versions
	// older than 1.18. In addition, system cert path can be overridden by
	// environment variables on many OSes. Therefore, Amazon root CA is
	// downloaded here to be safe.
	//
	// AWS MemoryDB uses same CA as ElastiCache.
	case types.DatabaseTypeElastiCache,
		types.DatabaseTypeMemoryDB,
		types.DatabaseTypeDynamoDB:
		return []string{filepath.Join(s.cfg.DataDir, filepath.Base(amazonRootCA1URL))}, nil

	// Each Cloud SQL instance has its own CA.
	case types.DatabaseTypeCloudSQL:
		return []string{filepath.Join(s.cfg.DataDir, fmt.Sprintf("%v-root.pem", database.GetName()))}, nil

	case types.DatabaseTypeAzure:
		return []string{
			filepath.Join(s.cfg.DataDir, filepath.Base(azureCAURLBaltimore)),
			filepath.Join(s.cfg.DataDir, filepath.Base(azureCAURLDigiCert)),
		}, nil

	case types.DatabaseTypeAWSKeyspaces:
		return []string{filepath.Join(s.cfg.DataDir, filepath.Base(amazonKeyspacesCAURL))}, nil

	case types.DatabaseTypeMongoAtlas:
		return []string{
			filepath.Join(s.cfg.DataDir, filepath.Base(isrgRootX1URL)),
		}, nil
	}

	return nil, trace.BadParameter("%v doesn't support automatic CA download", database)
}

// saveCACert saves the downloaded certificate to the filesystem.
func (s *Server) saveCACert(filePath string, content []byte, version []byte) error {
	// Save CA contents.
	err := os.WriteFile(filePath, content, teleport.FileMaskOwnerOnly)
	if err != nil {
		return trace.Wrap(err)
	}

	// Save the CA version.
	err = os.WriteFile(filePath+versionFileSuffix, version, teleport.FileMaskOwnerOnly)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Debugf("Saved CA certificate %v.", filePath)
	return nil
}

// updateCACert updates the database CA contents if it has changed.
func (s *Server) updateCACert(ctx context.Context, database types.Database, filePath string) error {
	var contents []byte

	// Get the current CA version.
	version, err := s.cfg.CADownloader.GetVersion(ctx, database, filepath.Base(filePath))
	// If getting the CA version is not supported, download it.
	if trace.IsNotImplemented(err) {
		contents, version, err = s.cfg.CADownloader.Download(ctx, database, filepath.Base(filePath))
		if err != nil {
			return trace.Wrap(err)
		}
	} else if err != nil {
		return trace.Wrap(err)
	}

	equal, err := isCAVersionEqual(filePath, version)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if equal {
		s.log.Debugf("Database %q CA is up-to-date.", database.GetName())
		return nil
	}

	// Check if the CA contents were already downloaded. If not, download them.
	if contents == nil {
		contents, version, err = s.cfg.CADownloader.Download(ctx, database, filepath.Base(filePath))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = s.saveCACert(filePath, contents, version)
	if err != nil {
		return trace.Wrap(err)
	}

	s.log.Infof("Database %q CA updated.", database.GetName())
	return nil
}

// isCAVersionEqual compares the in disk version with the provided one.
func isCAVersionEqual(filePath string, version []byte) (bool, error) {
	currentVersion, err := os.ReadFile(filePath + versionFileSuffix)
	if err != nil {
		return false, trace.ConvertSystemError(err)
	}

	return bytes.Equal(currentVersion, version), nil
}

// CADownloader defines interface for cloud databases CA cert downloaders.
type CADownloader interface {
	// Download downloads CA certificate for the provided database instance.
	Download(context.Context, types.Database, string) ([]byte, []byte, error)
	// GetVersion returns the CA version for the provided database.
	GetVersion(context.Context, types.Database, string) ([]byte, error)
}

type realDownloader struct {
	// httpClient is the HTTP client used to download CA certificates.
	httpClient *http.Client
	// sqladminClient is the Cloud SQL Admin API service used to download CA
	// certificates.
	sqlAdminClient gcp.SQLAdminClient
}

// NewRealDownloader returns real cloud database CA downloader.
func NewRealDownloader() CADownloader {
	return &realDownloader{
		httpClient: http.DefaultClient,
	}
}

// Download downloads CA certificate for the provided cloud database instance.
func (d *realDownloader) Download(ctx context.Context, database types.Database, hint string) ([]byte, []byte, error) {
	switch database.GetType() {
	case types.DatabaseTypeRDS:
		return d.downloadFromURL(rdsCAURLForDatabase(database))
	case types.DatabaseTypeRedshift,
		types.DatabaseTypeRedshiftServerless:
		return d.downloadFromURL(redshiftCAURLForDatabase(database))
	case types.DatabaseTypeElastiCache,
		types.DatabaseTypeMemoryDB,
		types.DatabaseTypeDynamoDB:
		return d.downloadFromURL(amazonRootCA1URL)
	case types.DatabaseTypeCloudSQL:
		return d.downloadForCloudSQL(ctx, database)
	case types.DatabaseTypeAzure:
		if strings.HasSuffix(azureCAURLBaltimore, hint) {
			return d.downloadFromURL(azureCAURLBaltimore)
		} else if strings.HasSuffix(azureCAURLDigiCert, hint) {
			return d.downloadFromURL(azureCAURLDigiCert)
		}
		return nil, nil, trace.BadParameter("unknown Azure CA %q", hint)
	case types.DatabaseTypeAWSKeyspaces:
		return d.downloadFromURL(amazonKeyspacesCAURL)
	case types.DatabaseTypeMongoAtlas:
		return d.downloadFromURL(isrgRootX1URL)
	}
	return nil, nil, trace.BadParameter("%v doesn't support automatic CA download", database)
}

// GetVersion returns the CA version for the provided database.
func (d *realDownloader) GetVersion(ctx context.Context, database types.Database, hint string) ([]byte, error) {
	switch database.GetType() {
	case types.DatabaseTypeRDS:
		return d.getVersionFromURL(database, rdsCAURLForDatabase(database))
	case types.DatabaseTypeRedshift,
		types.DatabaseTypeRedshiftServerless:
		return d.getVersionFromURL(database, redshiftCAURLForDatabase(database))
	case types.DatabaseTypeElastiCache,
		types.DatabaseTypeMemoryDB:
		return d.getVersionFromURL(database, amazonRootCA1URL)
	case types.DatabaseTypeAzure:
		if strings.HasSuffix(azureCAURLBaltimore, hint) {
			return d.getVersionFromURL(database, azureCAURLBaltimore)
		} else if strings.HasSuffix(azureCAURLDigiCert, hint) {
			return d.getVersionFromURL(database, azureCAURLDigiCert)
		}
		return nil, trace.BadParameter("unknown Azure CA %q", hint)
	case types.DatabaseTypeAWSKeyspaces:
		return d.getVersionFromURL(database, amazonKeyspacesCAURL)
	case types.DatabaseTypeMongoAtlas:
		return d.getVersionFromURL(database, isrgRootX1URL)
	}

	return nil, trace.NotImplemented("%v doesn't support fetching CA version", database)
}

// downloadFromURL downloads root certificate from the provided URL.
func (d *realDownloader) downloadFromURL(downloadURL string) ([]byte, []byte, error) {
	resp, err := d.httpClient.Get(downloadURL)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, trace.BadParameter("status code %v when fetching from %q",
			resp.StatusCode, downloadURL)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// If there is a http response and it does have the "etag" header present,
	// use it as the CA version.
	if resp.Header.Get("ETag") != "" {
		return bytes, []byte(resp.Header.Get("ETag")), nil
	}

	// Otherwise, hash the contents and return it as the version.
	hash := sha256.Sum256(bytes)
	return bytes, hash[:], nil
}

// downloadForCloudSQL downloads root certificate for the provided Cloud SQL
// instance.
//
// This database service GCP IAM role should have "cloudsql.instances.get"
// permission in order for this to work.
func (d *realDownloader) downloadForCloudSQL(ctx context.Context, database types.Database) ([]byte, []byte, error) {
	cl, err := d.getSQLAdminClient(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	instance, err := cl.GetDatabaseInstance(ctx, database)
	if err != nil {
		return nil, nil, trace.BadParameter(cloudSQLDownloadError, database.GetName(),
			err, database.GetGCP().InstanceID)
	}

	if instance.ServerCaCert != nil {
		return []byte(instance.ServerCaCert.Cert), []byte(instance.ServerCaCert.Sha1Fingerprint), nil
	}

	return nil, nil, trace.NotFound("Cloud SQL instance %v does not contain server CA certificate info: %v",
		database, instance)
}

// getVersionFromURL fetches the CA version from the URL without downloading it.
// If the CA download is required it returns trace.NotImplementedError.
func (d *realDownloader) getVersionFromURL(database types.Database, url string) ([]byte, error) {
	resp, err := d.httpClient.Head(url)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("ETag") != "" {
		return []byte(resp.Header.Get("ETag")), nil
	}

	return nil, trace.NotImplemented("%v doesn't support fetching CA version", database)
}

// getSQLAdminClient returns the client provided on the struct initialization,
// otherwise init a new one.
func (d *realDownloader) getSQLAdminClient(ctx context.Context) (gcp.SQLAdminClient, error) {
	if d.sqlAdminClient != nil {
		return d.sqlAdminClient, nil
	}

	return gcp.NewSQLAdminClient(ctx)
}

// rdsCAURLForDatabase returns root certificate download URL based on the region
// of the provided RDS server instance.
//
// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html
func rdsCAURLForDatabase(database types.Database) string {
	region := database.GetAWS().Region

	switch {
	case awsutils.IsCNRegion(region):
		return fmt.Sprintf(rdsCNRegionCAURLTemplate, region, region)

	case awsutils.IsUSGovRegion(region):
		return fmt.Sprintf(rdsUSGovRegionCAURLTemplate, region, region)

	default:
		return fmt.Sprintf(rdsDefaultCAURLTemplate, region, region)
	}
}

// redshiftCAURLForDatabase returns root certificate download URL based on the region
// of the provided RDS server instance.
func redshiftCAURLForDatabase(database types.Database) string {
	if awsutils.IsCNRegion(database.GetAWS().Region) {
		return redshiftCNRegionCAURL
	}
	return redshiftDefaultCAURL
}

const (
	// caRenewInterval is the interval that the cloud-hosted database CAs need
	// to be renewed.
	caRenewInterval = 24 * time.Hour
	// versionFileSuffix is the suffix for the file that contains the
	// resource version of the CA certificate.
	versionFileSuffix = ".version"
	// rdsDefaultCAURLTemplate is the string format template that creates URLs
	// for region based RDS CA bundles.
	//
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html
	rdsDefaultCAURLTemplate = "https://truststore.pki.rds.amazonaws.com/%s/%s-bundle.pem"
	// rdsUSGovRegionCAURLTemplate is the string format template that creates URLs
	// for region based RDS CA bundles for AWS US GovCloud regions
	//
	// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html
	rdsUSGovRegionCAURLTemplate = "https://truststore.pki.us-gov-west-1.rds.amazonaws.com/%s/%s-bundle.pem"
	// rdsCNRegionCAURLTemplate is the string format template that creates URLs
	// for region based RDS CA bundles for AWS China regions.
	//
	// https://docs.amazonaws.cn/en_us/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html
	rdsCNRegionCAURLTemplate = "https://rds-truststore.s3.cn-north-1.amazonaws.com.cn/%s/%s-bundle.pem"
	// redshiftDefaultCAURL is the Redshift CA bundle download URL.
	//
	// https://docs.aws.amazon.com/redshift/latest/mgmt/connecting-ssl-support.html
	redshiftDefaultCAURL = "https://s3.amazonaws.com/redshift-downloads/amazon-trust-ca-bundle.crt"
	// redshiftDefaultCAURL is the Redshift CA bundle download URL for AWS
	// China regions.
	//
	// https://docs.amazonaws.cn/redshift/latest/mgmt/connecting-ssl-support.html
	redshiftCNRegionCAURL = "https://s3.cn-north-1.amazonaws.com.cn/redshift-downloads-cn/amazon-trust-ca-bundle.crt"
	// amazonRootCA1URL is the root CA for many Amazon websites and services.
	//
	// https://www.amazontrust.com/repository/
	amazonRootCA1URL = "https://www.amazontrust.com/repository/AmazonRootCA1.pem"

	// azureCAURLBaltimore is the URL of the CA certificate for validating certificates
	// presented by Azure hosted databases. See:
	//
	// https://docs.microsoft.com/en-us/azure/postgresql/concepts-ssl-connection-security
	// https://docs.microsoft.com/en-us/azure/mysql/howto-configure-ssl
	azureCAURLBaltimore = "https://www.digicert.com/CACerts/BaltimoreCyberTrustRoot.crt.pem"
	// azureCAURLDigiCert is the URL of the new CA certificate for validating
	// certificates presented by Azure hosted databases.
	azureCAURLDigiCert = "https://cacerts.digicert.com/DigiCertGlobalRootG2.crt.pem"

	// amazonKeyspacesCAURL is the URL of the CA certificate for validating certificates
	// presented by AWS Keyspace. See:
	// https://docs.aws.amazon.com/keyspaces/latest/devguide/using_go_driver.html
	amazonKeyspacesCAURL = "https://certs.secureserver.net/repository/sf-class2-root.crt"

	// isrgRootX1URL is the URL to download ISRG Root X1 CA for Let's Encrypt. See:
	// https://letsencrypt.org/certificates/
	//
	// MongoDB Atlas uses certificates signed by Let's Encrypt:
	// https://www.mongodb.com/docs/atlas/reference/faq/security/#which-certificate-authority-signs-mongodb-atlas-tls-certificates-
	isrgRootX1URL = "https://letsencrypt.org/certs/isrgrootx1.pem"

	// cloudSQLDownloadError is the error message that gets returned when
	// we failed to download root certificate for Cloud SQL instance.
	cloudSQLDownloadError = `Could not download Cloud SQL CA certificate for database %v due to the following error:

    %v

To correct the error you can try the following:

  * Make sure this database service has "Cloud SQL Viewer" GCP IAM role, or
    "cloudsql.instances.get" IAM permission.

  * Download root certificate for your Cloud SQL instance %q manually and set
    it in the database configuration using "ca_cert_file" configuration field.`
)
