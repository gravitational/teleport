/*
Copyright 2020 Gravitational, Inc.

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
	"context"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

func (s *Server) initRDSRootCert(ctx context.Context, server services.DatabaseServer) error {
	// If this is not an AWS database, or CA was set explicitly, or it was
	// already loaded, then nothing to do.
	if !server.IsAWS() || len(server.GetCA()) != 0 || len(s.rdsCACerts[server.GetRegion()]) != 0 {
		return nil
	}
	// This is a RDS/Aurora instance and CA certificate wasn't explicitly
	// provided, so try to download it from AWS (or see if it's already
	// been downloaded).
	downloadURL := rdsDefaultCAURL
	if u, ok := rdsCAURLs[server.GetRegion()]; ok {
		downloadURL = u
	}
	bytes, err := s.ensureRDSRootCert(downloadURL)
	if err != nil {
		return trace.Wrap(err)
	}
	// Make sure the cert we got is valid just in case.
	_, err = tlsca.ParseCertificatePEM(bytes)
	if err != nil {
		return trace.Wrap(err, "RDS root certificate for %v doesn't appear to be a valid x509 certificate: %s",
			server, bytes)
	}
	s.rdsCACerts[server.GetRegion()] = bytes
	return nil
}

func (s *Server) ensureRDSRootCert(downloadURL string) ([]byte, error) {
	// The downloaded CA resides in the data dir under the same filename e.g.
	//   /var/lib/teleport/rds-ca-2019-root-pem
	filePath := filepath.Join(s.cfg.DataDir, filepath.Base(downloadURL))
	// Check if we already have it.
	_, err := utils.StatFile(filePath)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// It's already downloaded.
	if err == nil {
		s.log.Infof("Loaded RDS certificate %v.", filePath)
		return ioutil.ReadFile(filePath)
	}
	// Otherwise download it.
	return s.downloadRDSRootCert(downloadURL, filePath)
}

func (s *Server) downloadRDSRootCert(downloadURL, filePath string) ([]byte, error) {
	s.log.Infof("Downloading RDS certificate %v.", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("status code %v when fetching from %q",
			resp.StatusCode, downloadURL)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = ioutil.WriteFile(filePath, bytes, teleport.FileMaskOwnerOnly)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s.log.Infof("Saved RDS certificate %v.", filePath)
	return bytes, nil
}

var (
	// rdsDefaultCAURL is the URL of the default RDS root certificate that
	// works for all regions except the ones specified below.
	//
	// See https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html
	// for details.
	rdsDefaultCAURL = "https://s3.amazonaws.com/rds-downloads/rds-ca-2019-root.pem"
	// rdsCAURLs maps opt-in AWS regions to URLs of their RDS root
	// certificates.
	rdsCAURLs = map[string]string{
		"af-south-1":    "https://s3.amazonaws.com/rds-downloads/rds-ca-af-south-1-2019-root.pem",
		"ap-east-1":     "https://s3.amazonaws.com/rds-downloads/rds-ca-ap-east-1-2019-root.pem",
		"eu-south-1":    "https://s3.amazonaws.com/rds-downloads/rds-ca-eu-south-1-2019-root.pem",
		"me-south-1":    "https://s3.amazonaws.com/rds-downloads/rds-ca-me-south-1-2019-root.pem",
		"us-gov-east-1": "https://s3.us-gov-west-1.amazonaws.com/rds-downloads/rds-ca-us-gov-east-1-2017-root.pem",
		"us-gov-west-1": "https://s3.us-gov-west-1.amazonaws.com/rds-downloads/rds-ca-us-gov-west-1-2017-root.pem",
	}
)
