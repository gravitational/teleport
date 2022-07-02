/*
Copyright 2022 Gravitational, Inc.

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

package main

import (
	"fmt"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reporting"

	"github.com/gravitational/trace"
)

// recordLicenseStatus gets the license status and stores it in the tsh home directory.
func recordLicenseStatus(cf *CLIConf) error {
	profile, _, err := client.Status(cf.HomePath, cf.Proxy)
	if err != nil {
		return trace.Wrap(err)
	}
	if profile == nil {
		return nil
	}
	teleportClient, err := makeClient(cf, true)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyClient, err := teleportClient.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	authClient, err := proxyClient.ConnectToCurrentCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, ok := authClient.(*auth.Client)
	if !ok {
		trace.BadParameter("expected *auth.Client, got: %T", clt)
	}
	licenseStatus, err := reporting.GetLicenseStatus(cf.Context, clt)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := reporting.WriteLicenseStatus(cf.HomePath, profile.Name, licenseStatus); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// displayLicenseWarnings displays license out of compliance warnings.
func displayLicenseWarnings(cf *CLIConf) error {
	profile, _, err := client.Status(cf.HomePath, cf.Proxy)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if profile == nil {
		return nil
	}
	warnings, err := reporting.GetLicenseWarnings(cf.HomePath, profile.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, warning := range warnings {
		fmt.Println(warning)
	}
	return nil
}
