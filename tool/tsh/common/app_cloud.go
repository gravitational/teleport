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

package common

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
)

func defaultValue[t any]() t {
	var def t
	return def
}

// pickCloudApp will attempt to find an active cloud app, automatically logging the user to the selected application if possible.
func pickCloudApp[cloudApp any](cf *CLIConf, cloudFriendlyName string, matchRouteToApp func(tlsca.RouteToApp) bool, newCloudApp func(cf *CLIConf, profile *client.ProfileStatus, appRoute tlsca.RouteToApp) (cloudApp, error)) (cloudApp, error) {
	app, needLogin, err := pickActiveCloudApp[cloudApp](cf, cloudFriendlyName, matchRouteToApp, newCloudApp)
	if err != nil {
		if !needLogin {
			return defaultValue[cloudApp](), trace.Wrap(err)
		}
		log.WithError(err).Debugf("Failed to pick an active %v app, attempting to login into app %q", cloudFriendlyName, cf.AppName)
		quiet := cf.Quiet
		cf.Quiet = true
		errLogin := onAppLogin(cf)
		cf.Quiet = quiet
		if errLogin != nil {
			log.WithError(errLogin).Debugf("App login attempt failed")
			// combine errors
			return defaultValue[cloudApp](), trace.NewAggregate(err, errLogin)
		}
		// another attempt
		app, _, err = pickActiveCloudApp[cloudApp](cf, cloudFriendlyName, matchRouteToApp, newCloudApp)
		return app, trace.Wrap(err)
	}
	return app, nil
}

func pickActiveCloudApp[cloudApp any](cf *CLIConf, cloudFriendlyName string, matchRouteToApp func(tlsca.RouteToApp) bool, newCloudApp func(cf *CLIConf, profile *client.ProfileStatus, appRoute tlsca.RouteToApp) (cloudApp, error)) (cApp cloudApp, needLogin bool, err error) {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return defaultValue[cloudApp](), false, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		if cf.AppName == "" {
			return defaultValue[cloudApp](), false, trace.NotFound("please login to %v app using 'tsh apps login' first", cloudFriendlyName)
		}
		return defaultValue[cloudApp](), true, trace.NotFound("please login to %v app using 'tsh apps login %v' first", cloudFriendlyName, cf.AppName)
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return defaultValue[cloudApp](), true, trace.NotFound("please login to %v app using 'tsh apps login %v' first", cloudFriendlyName, name)
			}
			return defaultValue[cloudApp](), false, trace.Wrap(err)
		}
		if !matchRouteToApp(*app) {
			return defaultValue[cloudApp](), false, trace.BadParameter(
				"selected app %q is not an %v application", name, cloudFriendlyName,
			)
		}

		cApp, err := newCloudApp(cf, profile, *app)
		return cApp, false, trace.Wrap(err)
	}

	filteredApps := filterApps(matchRouteToApp, profile.Apps)
	if len(filteredApps) == 0 {
		// no app name to use for attempted login.
		return defaultValue[cloudApp](), false, trace.NotFound("please login to %v App using 'tsh apps login' first", cloudFriendlyName)
	}
	if len(filteredApps) > 1 {
		names := strings.Join(getAppNames(filteredApps), ", ")
		return defaultValue[cloudApp](), false, trace.BadParameter(
			"multiple %v apps are available (%v), please specify one using --app CLI argument", cloudFriendlyName, names,
		)
	}
	cApp, err = newCloudApp(cf, profile, filteredApps[0])
	return cApp, false, trace.Wrap(err)
}

func filterApps(matchRouteToApp func(tlsca.RouteToApp) bool, apps []tlsca.RouteToApp) []tlsca.RouteToApp {
	var out []tlsca.RouteToApp
	for _, app := range apps {
		if matchRouteToApp(app) {
			out = append(out, app)
		}
	}
	return out
}

func getAppNames(apps []tlsca.RouteToApp) []string {
	var out []string
	for _, app := range apps {
		out = append(out, app.Name)
	}
	return out
}

func findApp(apps []tlsca.RouteToApp, name string) (*tlsca.RouteToApp, error) {
	for _, app := range apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, trace.NotFound("failed to find app with %q name", name)
}
