// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
)

// cloudApp is a dummy interface shared between awsApp, azureApp and gcpApp
type cloudApp interface {
	dummyCloudAppMethod()
}

// cloudAppImpl automatically implements cloudApp when inherited
type cloudAppImpl struct {
}

func (_ cloudAppImpl) dummyCloudAppMethod() {
}

// cloudAppInfo contains common information for given cloudApp
type cloudAppInfo struct {
	// matchRouteToApp returns true if given tlsca.RouteToApp is recognized as a given cloud app kind.
	matchRouteToApp func(tlsca.RouteToApp) bool
	// newCloudApp creates a new instance of cloudApp
	newCloudApp func(cf *CLIConf, profile *client.ProfileStatus, appRoute tlsca.RouteToApp) (cloudApp, error)
	// cloudFriendlyName is a friendly name for given cloud
	cloudFriendlyName string
}

func (info *cloudAppInfo) pickActiveCloudApp(cf *CLIConf) (cloudApp, error) {
	app, needLogin, err := info.pickActiveCloudAppNoRetry(cf)
	if err != nil {
		if !needLogin {
			return nil, trace.Wrap(err)
		}
		log.WithError(err).Debugf("Failed to pick an active %v app, attempting to login into app %q", info.cloudFriendlyName, cf.AppName)
		errLogin := onAppLogin(cf)
		if errLogin != nil {
			log.WithError(errLogin).Debugf("App login attempt failed")
			// combine errors
			return nil, trace.NewAggregate(err, errLogin)
		}
		// another attempt
		app, _, err = info.pickActiveCloudAppNoRetry(cf)
		return app, trace.Wrap(err)
	}
	return app, nil
}

func (info *cloudAppInfo) pickActiveCloudAppNoRetry(cf *CLIConf) (cApp cloudApp, needLogin bool, err error) {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return nil, false, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		if cf.AppName == "" {
			return nil, false, trace.NotFound("please login to %v app using 'tsh apps login' first", info.cloudFriendlyName)
		}
		return nil, true, trace.NotFound("please login to %v app using 'tsh apps login %v' first", info.cloudFriendlyName, cf.AppName)
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, true, trace.NotFound("please login to %v app using 'tsh apps login %v' first", info.cloudFriendlyName, name)
			}
			return nil, false, trace.Wrap(err)
		}
		if !info.matchRouteToApp(*app) {
			return nil, false, trace.BadParameter(
				"selected app %q is not an %v application", name, info.cloudFriendlyName,
			)
		}

		cApp, err := info.newCloudApp(cf, profile, *app)
		return cApp, false, trace.Wrap(err)
	}

	filteredApps := filterApps(info.matchRouteToApp, profile.Apps)
	if len(filteredApps) == 0 {
		// no app name to use for attempted login.
		return nil, false, trace.NotFound("please login to %v App using 'tsh apps login' first", info.cloudFriendlyName)
	}
	if len(filteredApps) > 1 {
		names := strings.Join(getAppNames(filteredApps), ", ")
		return nil, false, trace.BadParameter(
			"multiple %v apps are available (%v), please specify one using --app CLI argument", info.cloudFriendlyName, names,
		)
	}
	cApp, err = info.newCloudApp(cf, profile, filteredApps[0])
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
