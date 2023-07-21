/*
Copyright 2020-2021 Gravitational, Inc.

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

package db

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

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
	// CA certificate may be set explicitly via configuration.
	if len(database.GetCA()) != 0 {
		return false
	}
	// Can only download it for cloud-hosted instances.
	switch database.GetType() {
	case types.DatabaseTypeRDS,
		types.DatabaseTypeRedshift,
		types.DatabaseTypeElastiCache,
		types.DatabaseTypeMemoryDB,
		types.DatabaseTypeAWSKeyspaces,
		types.DatabaseTypeCloudSQL,
		types.DatabaseTypeAzure:
		return true
	default:
		return false
	}
}

// getCACerts returns automatically downloaded root certificate for the provided
// cloud database instance.
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
// The cert can already be cached in the filesystem, otherwise we will attempt
// to download it.
func (s *Server) getCACert(ctx context.Context, database types.Database, filePath string) ([]byte, error) {
	// Check if we already have it.
	_, err := utils.StatFile(filePath)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// It's already downloaded.
	if err == nil {
		s.log.Debugf("Loaded CA certificate %v.", filePath)
		return os.ReadFile(filePath)
	}
	// Otherwise download it.
	s.log.Debugf("Downloading CA certificate for %v.", database)
	bytes, err := s.cfg.CADownloader.Download(ctx, database, filepath.Base(filePath))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Save to the filesystem.
	err = os.WriteFile(filePath, bytes, teleport.FileMaskOwnerOnly)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Debugf("Saved CA certificate %v.", filePath)
	return bytes, nil
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
	case types.DatabaseTypeRedshift:
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
		types.DatabaseTypeMemoryDB:
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
	}

	return nil, trace.BadParameter("%v doesn't support automatic CA download", database)
}

// CADownloader defines interface for cloud databases CA cert downloaders.
type CADownloader interface {
	// Download downloads CA certificate for the provided database instance.
	Download(context.Context, types.Database, string) ([]byte, error)
}

type realDownloader struct {
}

// NewRealDownloader returns real cloud database CA downloader.
func NewRealDownloader() CADownloader {
	return &realDownloader{}
}

// Download downloads CA certificate for the provided cloud database instance.
func (d *realDownloader) Download(ctx context.Context, database types.Database, hint string) ([]byte, error) {
	switch database.GetType() {
	case types.DatabaseTypeRDS:
		return d.downloadFromURL(rdsCAURLForDatabase(database))
	case types.DatabaseTypeRedshift:
		return d.downloadFromURL(redshiftCAURLForDatabase(database))
	case types.DatabaseTypeElastiCache,
		types.DatabaseTypeMemoryDB:
		return d.downloadFromURL(amazonRootCA1URL)
	case types.DatabaseTypeCloudSQL:
		return d.downloadForCloudSQL(ctx, database)
	case types.DatabaseTypeAzure:
		if strings.HasSuffix(azureCAURLBaltimore, hint) {
			return d.downloadFromURL(azureCAURLBaltimore)
		} else if strings.HasSuffix(azureCAURLDigiCert, hint) {
			return d.downloadFromURL(azureCAURLDigiCert)
		}
		return nil, trace.BadParameter("unknown Azure CA %q", hint)
	case types.DatabaseTypeAWSKeyspaces:
		return d.downloadFromURL(amazonKeyspacesCAURL)
	}
	return nil, trace.BadParameter("%v doesn't support automatic CA download", database)
}

// downloadFromURL downloads root certificate from the provided URL.
func (d *realDownloader) downloadFromURL(downloadURL string) ([]byte, error) {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("status code %v when fetching from %q",
			resp.StatusCode, downloadURL)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

// downloadForCloudSQL downloads root certificate for the provided Cloud SQL
// instance.
//
// This database service GCP IAM role should have "cloudsql.instances.get"
// permission in order for this to work.
func (d *realDownloader) downloadForCloudSQL(ctx context.Context, database types.Database) ([]byte, error) {
	sqladminService, err := sqladmin.NewService(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instance, err := sqladmin.NewInstancesService(sqladminService).Get(
		database.GetGCP().ProjectID, database.GetGCP().InstanceID).Context(ctx).Do()
	if err != nil {
		return nil, trace.BadParameter(cloudSQLDownloadError, database.GetName(),
			err, database.GetGCP().InstanceID)
	}
	if instance.ServerCaCert != nil {
		return []byte(instance.ServerCaCert.Cert), nil
	}
	return nil, trace.NotFound("Cloud SQL instance %v does not contain server CA certificate info: %v",
		database, instance)
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
