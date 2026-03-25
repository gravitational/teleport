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
	"regexp"
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/aws/awsconfigfile"
	"github.com/gravitational/teleport/lib/client"
)

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

		// Fetch all app resources from the cluster.
		resources, err = apiclient.GetAllUnifiedResources(cf.Context, clt.AuthClient, &proto.ListUnifiedResourcesRequest{
			Kinds:         []string{types.KindApp},
			IncludeLogins: true, // This enables permission set filtering for AWS IC apps
		})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	icApps := filterAWSIdentityCenterApps(resources)

	configPath, err := awsconfigfile.AWSConfigFilePath()
	if err != nil {
		return trace.Wrap(err)
	}

	// Write into AWS config file.
	writtenProfiles, err := writeAWSConfig(configPath, cf.AWSSSORegion, icApps, cf.DryRun, cf.Stdout())
	if err != nil {
		return trace.Wrap(err)
	}

	// Output summary to user if not in dry-run mode.
	if !cf.DryRun {
		writeAWSProfileSummary(cf.Stdout(), configPath, writtenProfiles)
	}

	return nil
}

func filterAWSIdentityCenterApps(resources types.EnrichedResources) []types.Application {
	var apps []types.Application
	for _, resource := range resources {
		app, ok := resource.ResourceWithLabels.(types.AppServer)
		if !ok {
			continue
		}
		if app.GetApp().GetIdentityCenter() == nil {
			continue
		}
		apps = append(apps, app.GetApp())
	}
	return apps
}

func writeAWSConfig(configPath, ssoRegionOverride string, identityCenterApps []types.Application, dryRun bool, out io.Writer) ([]awsconfigfile.SSOProfile, error) {
	sessionMap := make(map[string]awsconfigfile.SSOSession)
	var profiles []awsconfigfile.SSOProfile

	for _, app := range identityCenterApps {
		startURL := extractAWSStartURL(app.GetURI())
		sessionName := extractAWSSessionName(startURL)

		if _, ok := sessionMap[sessionName]; !ok {
			// Use the explicit flag if provided, otherwise read from the label
			// set by the Identity Center sync.
			region := ssoRegionOverride
			if region == "" {
				region, _ = app.GetLabel(types.AWSSSORegionLabel)
			}
			if region == "" {
				return nil, trace.BadParameter("could not determine SSO region for app %q; use --aws-sso-region to specify it", app.GetName())
			}
			sessionMap[sessionName] = awsconfigfile.SSOSession{
				Name:     sessionName,
				StartURL: startURL,
				Region:   region,
			}
		}

		awsIC := app.GetIdentityCenter()

		accountName, _ := app.GetLabel(types.AWSAccountNameLabel)
		if accountName == "" {
			accountName = awsIC.AccountID
		}

		// Prepare AWS profile for the combination of each permission set and account.
		for _, ps := range awsIC.PermissionSets {
			profileName := formatAWSProfileName(accountName, ps.Name)
			profiles = append(profiles, awsconfigfile.SSOProfile{
				Name:      profileName,
				Session:   sessionName,
				AccountID: awsIC.AccountID,
				RoleName:  ps.Name,
				Account:   accountName,
			})
		}
	}

	sessions := make([]awsconfigfile.SSOSession, 0, len(sessionMap))
	for _, s := range sessionMap {
		sessions = append(sessions, s)
	}

	if dryRun {
		if err := awsconfigfile.PrintSSOConfig(configPath, profiles, sessions, out); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := awsconfigfile.WriteSSOConfig(configPath, profiles, sessions); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return profiles, nil
}

func writeAWSProfileSummary(w io.Writer, configPath string, profiles []awsconfigfile.SSOProfile) {
	if len(profiles) > 0 {
		fmt.Fprintf(w, "AWS configuration updated at: %s\n", configPath)
		fmt.Fprintln(w)

		table := asciitable.MakeTable([]string{"Profile", "Account", "Account ID", "Role", "SSO Session"})
		for _, p := range profiles {
			table.AddRow([]string{p.Name, p.Account, p.AccountID, p.RoleName, p.Session})
		}
		table.WriteTo(w)
		fmt.Fprintln(w)

		fmt.Fprintf(w, "To use these profiles, set the AWS_PROFILE environment variable and authenticate with AWS. Example:\n")
		fmt.Fprintf(w, "  export AWS_PROFILE=%s\n", profiles[0].Name)
		fmt.Fprintf(w, "  aws sso login\n")
		fmt.Fprintln(w)
		fmt.Fprintf(w, "You can now run AWS commands. Example:\n")
		fmt.Fprintf(w, "  aws s3 ls\n")
		fmt.Fprintln(w)
	} else {
		fmt.Fprintln(w, "No AWS Identity Center accounts or roles available for the current user.")
	}
}

// awsProfileNameRegex is used to sanitize AWS profile names. The AWS CLI supports relatively
// arbitrary characters in profile names, but since these are derived from AWS IAM roles and Account
// names remotely, we restrict them heavily to prevent INI injection in configuration blocks.
var awsProfileNameRegex = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)

func formatAWSProfileName(accountName, roleName string) string {
	accName := strings.ReplaceAll(accountName, " ", "-")
	rName := strings.ReplaceAll(roleName, " ", "-")
	combined := fmt.Sprintf("teleport-awsic-%s-%s", accName, rName)
	return strings.ToLower(awsProfileNameRegex.ReplaceAllString(combined, ""))
}

func extractAWSStartURL(rawURL string) string {
	// Identity Center Start URLs use '#' to separate the portal base URL from the specific console path.
	// Standard: https://<subdomain>.awsapps.com/start/#/console...
	// GovCloud: https://start.us-gov-home.awsapps.com/directory/<idSource>#/console...
	if index := strings.Index(rawURL, "#"); index != -1 {
		return strings.TrimSuffix(rawURL[:index], "/")
	}

	// Fallback to legacy behavior if anchor is missing but /start/ is present.
	if index := strings.Index(rawURL, "/start/"); index != -1 {
		return rawURL[:index+len("/start")]
	}

	return rawURL
}

func extractAWSSessionName(startURL string) string {
	// For GovCloud, the unique identifier is at the end of the directory path.
	// Pattern: https://start.us-gov-home.awsapps.com/directory/<idSource>
	if index := strings.LastIndex(startURL, "/directory/"); index != -1 {
		id := startURL[index+len("/directory/"):]
		if id != "" {
			return "teleport-" + id
		}
	}
	// For standard partition, the unique identifier is the subdomain.
	// Pattern: https://<idSource>.awsapps.com/start
	raw := strings.TrimPrefix(startURL, "https://")
	if dotIndex := strings.Index(raw, "."); dotIndex != -1 {
		return "teleport-" + raw[:dotIndex]
	}
	// Rare: fallback to a hash of the URL if we can't find a subdomain to ensure uniqueness
	return fmt.Sprintf("teleport-%x", sha256.Sum256([]byte(startURL)))[:16]
}
