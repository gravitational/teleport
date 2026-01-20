/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"net/url"
	"os/exec"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
)

func onAppCurl(cf *CLIConf) error {
	if cf.AppName == "" {
		return trace.BadParameter("app name is required")
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var (
		clusterClient *client.ClusterClient
		appInfo       *appInfo
		app           types.Application
	)
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		profile, err := tc.ProfileStatus()
		if err != nil {
			return trace.Wrap(err)
		}

		clusterClient, err = tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}

		appInfo, err = getAppInfo(cf, clusterClient.AuthClient, profile, tc.SiteName, nil /*matchRouteToApp*/)
		if err != nil {
			return trace.Wrap(err)
		}

		app, err = appInfo.GetApp(cf.Context, clusterClient.AuthClient)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	// TODO(greedy52) make a proper function to filter out cloud apps
	if app.GetProtocol() != "HTTP" {
		return trace.BadParameter("unsupported protocol: %s", app.GetProtocol())
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := pickActiveApp(cf, profile.Apps); err != nil {
		// TODO(greedy52) do local proxy when necessary
		rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}

		appCertParams := client.ReissueParams{
			RouteToCluster: tc.SiteName,
			RouteToApp:     appInfo.RouteToApp,
			AccessRequests: appInfo.profile.ActiveRequests,
		}
		key, err := appLogin(cf.Context, clusterClient, rootClient, appCertParams)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := tc.LocalAgent().AddAppKeyRing(key); err != nil {
			return trace.Wrap(err)
		}

		profile, err = tc.ProfileStatus()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	proxyURL, err := url.JoinPath("https://"+tc.WebProxyAddr, cf.DatabaseUser)
	if err != nil {
		return trace.Wrap(err)
	}

	cmd := exec.CommandContext(cf.Context,
		"curl",
		append([]string{
			proxyURL,
			"--cert", profile.AppCertPath(tc.SiteName, appInfo.Name),
			"--key", profile.AppKeyPath(tc.SiteName, appInfo.Name),
		}, cf.AWSCommandArgs...)...,
	)
	cmd.Stdin = cf.Stdin()
	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	return trace.Wrap(cmd.Run())
}
