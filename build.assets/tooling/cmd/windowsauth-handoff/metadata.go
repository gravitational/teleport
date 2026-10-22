/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

var zipNamePattern = regexp.MustCompile(`^Signed_([A-Za-z0-9_-]+)\.zip$`)

// prepareMetadata contains only the CI metadata fields needed to build submission.json
type prepareMetadata struct {
	WindowsauthVersion string `json:"windowsauth_version"`
	SignedCABSHA256    string `json:"signed_cab_sha256"`
	CAB                struct {
		S3Key string `json:"s3_key"`
	} `json:"cab"`
}

// downloadReceipt saved locally as download.json, upload consumes it to set the
// downloader's identity and download time
type downloadReceipt struct {
	CABS3Key              string    `json:"cab_s3_key"`
	CABDownloadedAt       time.Time `json:"cab_downloaded_at"`
	CABDownloadedByAWSARN string    `json:"cab_downloaded_by_aws_arn"`
}

// microsoftSubmission identifies the Microsoft's returned zip and the exact s3
// object uploaded by Operator
type microsoftSubmission struct {
	ID                     string `json:"id"`
	ReturnedZIPFilename    string `json:"returned_zip_filename"`
	ReturnedZIPSHA256      string `json:"returned_zip_sha256"`
	ReturnedZIPS3Key       string `json:"returned_zip_s3_key"`
	ReturnedZIPS3VersionID string `json:"returned_zip_s3_version_id"`
}

// submissionRecord is the companion submission.json, to be consumed by finalize
// describes the handoff, does not establish provenance
type submissionRecord struct {
	SchemaVersion         int                 `json:"schema_version"`
	WindowsauthVersion    string              `json:"windowsauth_version"`
	PrepareCABS3Key       string              `json:"prepare_cab_s3_key"`
	PrepareCABSHA256      string              `json:"prepare_cab_sha256"`
	MicrosoftSubmission   microsoftSubmission `json:"microsoft_submission"`
	UploadedByAWSARN      string              `json:"uploaded_by_aws_arn"`
	CABDownloadedByAWSARN string              `json:"cab_downloaded_by_aws_arn"`
	CABDownloadedAt       time.Time           `json:"cab_downloaded_at"`
	UploadedAt            time.Time           `json:"uploaded_at"`
	OperatorNote          string              `json:"operator_note"`
}

func normalizeVersion(value string) (string, error) {
	version := value
	if strings.HasPrefix(version, "v") || strings.HasPrefix(version, "V") {
		version = version[1:]
	}
	parsed, err := semver.NewVersion(version)
	if err != nil || parsed.String() != version || parsed.Major < 0 || parsed.Minor < 0 || parsed.Patch < 0 || parsed.Metadata != "" {
		return "", trace.BadParameter("expected MAJOR.MINOR.PATCH with an optional prerelease suffix: %q", value)
	}
	return version, nil
}

func fileChecksum(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", trace.Wrap(err)
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func readJSON(filename string, value any) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(json.Unmarshal(data, value), "read %s", filename)
}

func writeJSON(filename string, value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data = append(data, '\n')

	return data, trace.Wrap(os.WriteFile(filename, data, 0600))
}
