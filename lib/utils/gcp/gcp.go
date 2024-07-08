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

package gcp

import (
	"context"
	"encoding/json"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/gravitational/trace"
	"golang.org/x/oauth2/google"
)

// SortedGCPServiceAccounts sorts service accounts by project and service account name.
type SortedGCPServiceAccounts []string

// Len returns the length of a list.
func (s SortedGCPServiceAccounts) Len() int {
	return len(s)
}

// Less compares items. Given two accounts, it first compares the project (i.e. what goes after @)
// and if they are equal proceeds to compare the service account name (what goes before @).
// Example of sorted list:
// - test-0@example-100200.iam.gserviceaccount.com
// - test-1@example-123456.iam.gserviceaccount.com
// - test-2@example-123456.iam.gserviceaccount.com
// - test-3@example-123456.iam.gserviceaccount.com
// - test-0@other-999999.iam.gserviceaccount.com
func (s SortedGCPServiceAccounts) Less(i, j int) bool {
	beforeI, afterI, _ := strings.Cut(s[i], "@")
	beforeJ, afterJ, _ := strings.Cut(s[j], "@")

	if afterI != afterJ {
		return afterI < afterJ
	}

	return beforeI < beforeJ
}

// Swap swaps two items in a list.
func (s SortedGCPServiceAccounts) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

const serviceAccountParentDomain = "iam.gserviceaccount.com"

func ProjectIDFromServiceAccountName(serviceAccount string) (string, error) {
	if serviceAccount == "" {
		return "", trace.BadParameter("invalid service account format: empty string received")
	}

	user, domain, found := strings.Cut(serviceAccount, "@")
	if !found {
		return "", trace.BadParameter("invalid service account format: missing @")
	}
	if user == "" {
		return "", trace.BadParameter("invalid service account format: empty user")
	}

	projectID, iamDomain, found := strings.Cut(domain, ".")
	if !found {
		return "", trace.BadParameter("invalid service account format: missing <project-id>.iam.gserviceaccount.com after @")
	}

	if projectID == "" {
		return "", trace.BadParameter("invalid service account format: missing project ID")
	}

	if iamDomain != serviceAccountParentDomain {
		return "", trace.BadParameter("invalid service account format: expected suffix %q, got %q", serviceAccountParentDomain, iamDomain)
	}

	return projectID, nil
}

func ValidateGCPServiceAccountName(serviceAccount string) error {
	_, err := ProjectIDFromServiceAccountName(serviceAccount)
	return err
}

// GetServiceAccountFromCredentials attempts to retrieve service account email
// from provided credentials.
func GetServiceAccountFromCredentials(credentials *google.Credentials) (string, error) {
	// When credentials JSON file is provided through either
	// GOOGLE_APPLICATION_CREDENTIALS env var or a well known file.
	if len(credentials.JSON) > 0 {
		sa, err := GetServiceAccountFromCredentialsJSON(credentials.JSON)
		return sa, trace.Wrap(err)
	}

	// No credentials from JSON files but using metadata endpoints when on
	// Google Compute Engine.
	if metadata.OnGCE() {
		email, err := metadata.EmailWithContext(context.Background(), "")
		return email, trace.Wrap(err)
	}

	return "", trace.NotImplemented("unknown environment for getting service account")
}

// GetServiceAccountFromCredentialsJSON attempts to retrieve service account
// email from provided credentials JSON.
func GetServiceAccountFromCredentialsJSON(credentialsJSON []byte) (string, error) {
	content := struct {
		// ClientEmail defines the service account email for service_account
		// credentials.
		//
		// Reference: https://google.aip.dev/auth/4112
		ClientEmail string `json:"client_email"`

		// ServiceAccountImpersonationURL is used for external
		// account_credentials (e.g. Workload Identity Federation) when using
		// service account personation.
		//
		// Reference: https://google.aip.dev/auth/4117
		ServiceAccountImpersonationURL string `json:"service_account_impersonation_url"`
	}{}

	if err := json.Unmarshal(credentialsJSON, &content); err != nil {
		return "", trace.Wrap(err)
	}

	if content.ClientEmail != "" {
		return content.ClientEmail, nil
	}

	// Format:
	// https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/$EMAIL:generateAccessToken
	if _, after, ok := strings.Cut(content.ServiceAccountImpersonationURL, "/serviceAccounts/"); ok {
		index := strings.LastIndex(after, serviceAccountParentDomain)
		if index < 0 {
			return "", trace.BadParameter("invalid service_account_impersonation_url %q", content.ServiceAccountImpersonationURL)
		}
		return after[:index+len(serviceAccountParentDomain)], nil
	}

	return "", trace.NotImplemented("unknown environment for getting service account")
}
