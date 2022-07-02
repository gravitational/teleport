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

package common

import (
	"fmt"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reporting"
	"github.com/gravitational/teleport/lib/service"

	"github.com/gravitational/trace"
)

// displayLicenseWarnings displays license out of compliance warnings.
func displayLicenseWarnings(config *service.Config) error {
	proxyAddr := ""
	if len(config.AuthServers) != 0 {
		proxyAddr = config.AuthServers[0].Addr
	}
	profile, _, err := client.Status(config.TeleportHome, proxyAddr)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if profile == nil {
		return nil
	}
	warnings, err := reporting.GetLicenseWarnings(config.TeleportHome, profile.Name)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, warning := range warnings {
		fmt.Println(warning)
	}
	return nil
}
