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
	"strings"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
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

	writtenSessions := make(map[string]struct{})

	for _, resource := range resources {
		app := resource.ResourceWithLabels.(types.AppServer).GetApp()
		awsIC := app.GetIdentityCenter()
		if awsIC == nil {
			continue
		}

		startURL := extractAWSStartURL(app.GetURI())
		sessionName := extractAWSSessionName(startURL)

		if _, ok := writtenSessions[sessionName]; !ok {
			if err := awsconfigfile.UpsertSSOSession(configPath, sessionName, startURL); err != nil {
				return trace.Wrap(err)
			}
			writtenSessions[sessionName] = struct{}{}
		}

		// fmt.Printf("aws ic url: %s\n", startURL)
		// fmt.Printf("aws ic accountid: %s\n", awsIC.AccountID)
		// accountName, _ := app.GetLabel("teleport.dev/account-name")
		// fmt.Printf("aws ic accountName: %s\n", accountName)

		// for _, ps := range awsIC.PermissionSets {
		// 	fmt.Printf("aws ic role: %s\n", ps.Name)
		// }
	}

	return nil
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
