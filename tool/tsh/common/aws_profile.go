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

package common

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/aws/awsconfigfile"
	"github.com/gravitational/teleport/lib/client"
)

type awsProfileInfo struct {
	profile    string
	account    string
	accountID  string
	role       string
	ssoSession string
}

// onAWSProfile generates AWS configuration for AWS Identity Center integration.
// It's a noop if there are no AWS Identity Center integrations.
func onAWSProfile(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var resources types.EnrichedResources
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clt, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clt.Close()

		resources, err = apiclient.GetAllUnifiedResources(cf.Context, clt.AuthClient, &proto.ListUnifiedResourcesRequest{
			Kinds:         []string{types.KindApp},
			IncludeLogins: true, // This enables permission set filtering

		})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	configPath, err := awsconfigfile.AWSConfigFilePath()
	if err != nil {
		return trace.Wrap(err)
	}

	writtenProfiles, err := writeAWSConfig(configPath, cf.AWSSSORegion, resources)
	if err != nil {
		return trace.Wrap(err)
	}

	writeAWSProfileSummary(cf.Stdout(), configPath, writtenProfiles)

	return nil
}

func writeAWSConfig(configPath, ssoRegion string, resources types.EnrichedResources) ([]awsProfileInfo, error) {
	writtenSessions := make(map[string]struct{})
	var writtenProfiles []awsProfileInfo

	for _, resource := range resources {
		app, ok := resource.ResourceWithLabels.(types.AppServer)
		if !ok {
			continue
		}
		awsIC := app.GetApp().GetIdentityCenter()
		if awsIC == nil {
			continue
		}

		startURL := extractAWSStartURL(app.GetApp().GetURI())
		sessionName := extractAWSSessionName(startURL)

		if _, ok := writtenSessions[sessionName]; !ok {
			if err := awsconfigfile.UpsertSSOSession(configPath, sessionName, startURL, ssoRegion); err != nil {
				return nil, trace.Wrap(err)
			}
			writtenSessions[sessionName] = struct{}{}
		}

		accountName, _ := app.GetApp().GetLabel("teleport.dev/account-name")
		if accountName == "" {
			accountName = awsIC.AccountID
		}

		for _, ps := range awsIC.PermissionSets {
			profileName := formatAWSProfileName(accountName, ps.Name)
			if err := awsconfigfile.UpsertSSOProfile(configPath, profileName, sessionName, awsIC.AccountID, ps.Name); err != nil {
				return nil, trace.Wrap(err)
			}
			writtenProfiles = append(writtenProfiles, awsProfileInfo{
				profile:    profileName,
				account:    accountName,
				accountID:  awsIC.AccountID,
				role:       ps.Name,
				ssoSession: sessionName,
			})
		}
	}
	return writtenProfiles, nil
}

func writeAWSProfileSummary(w io.Writer, configPath string, profiles []awsProfileInfo) {
	if len(profiles) > 0 {
		fmt.Fprintf(w, "AWS configuration updated at: %s\n", configPath)
		fmt.Fprintln(w)

		fmt.Fprintf(w, "To use these profiles, first authenticate with AWS. Example:\n")
		fmt.Fprintf(w, "  aws sso login --sso-session %s\n", profiles[0].ssoSession)
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Then set the AWS_PROFILE environment variable. Example:\n")
		fmt.Fprintf(w, "  export AWS_PROFILE=%s\n", profiles[0].profile)
		fmt.Fprintln(w)

		// Simple table format
		fmt.Fprintf(w, "%-40s %-20s %-15s %-15s %-20s\n", "Profile", "Account", "Account ID", "Role", "SSO Session")
		fmt.Fprintln(w, strings.Repeat("-", 114))
		for _, v := range profiles {
			fmt.Fprintf(w, "%-40s %-20s %-15s %-15s %-20s\n", v.profile, v.account, v.accountID, v.role, v.ssoSession)
		}
	} else {
		fmt.Fprintln(w, "No AWS Identity Center integrations found.")
	}
}

func formatAWSProfileName(accountName, roleName string) string {
	return strings.ToLower(fmt.Sprintf("teleport-awsic-%s-%s", accountName, roleName))
}

func extractAWSStartURL(rawURL string) string {
	// asssume the rawURL is like https://example.awsapps.com/start/#/console?param=value
	// the output would be https://example.awsapps.com/start
	if start := strings.Index(rawURL, "/#/"); start != -1 {
		return rawURL[:start]
	}
	return rawURL
}

func extractAWSSessionName(startURL string) string {
	// assume the startURL is like https://example.awsapps.com/start
	// the output would be "teleport-example"
	raw := strings.TrimPrefix(startURL, "https://")
	if dotIndex := strings.Index(raw, "."); dotIndex != -1 {
		return "teleport-" + raw[:dotIndex]
	}
	// Rare: fallback to a hash of the URL if we can't find a subdomain to ensure uniqueness
	return fmt.Sprintf("teleport-%x", sha256.Sum256([]byte(startURL)))[:16]
}
