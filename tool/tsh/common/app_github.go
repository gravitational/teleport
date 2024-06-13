/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"fmt"
	"slices"
	"sort"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
)

func getGitHubUsernameFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	usernames := profile.GitHubUsernames
	if len(usernames) == 0 {
		return "", trace.BadParameter("no GitHub username available, check your permissions")
	}

	reqUsername := cf.GitHubUsername

	// if flag is missing, try to find singleton identity; failing that, print available options.
	if reqUsername == "" {
		if len(usernames) == 1 {
			log.Infof("GitHub username %v is selected by default as it is the only username available for this app.", usernames[0])
			return usernames[0], nil
		}

		fmt.Println(formatCloudAppAccounts("GitHub Usernames", sort.StringSlice(usernames)))
		return "", trace.BadParameter("multiple GitHub usernames available, choose one with --github-username flag")
	}

	// exact match.
	if slices.Contains(usernames, reqUsername) {
		return reqUsername, nil
	}

	fmt.Println(formatCloudAppAccounts("GitHub Usernames", sort.StringSlice(usernames)))
	return "", trace.NotFound("failed to find the username matching %q", reqUsername)
}
