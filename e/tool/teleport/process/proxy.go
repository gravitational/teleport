/*
Copyright 2023 Gravitational, Inc.

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

package process

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/service"
)

// extendProxy extends the proxy with enterprise specific features.
func extendProxy(ossProcess *service.TeleportProcess, webPlugin *web.Plugin) error {
	if ossProcess.Config.Proxy.IdP.SAMLIdP.Enabled {
		// We have to wait for the proxy to be finished before we can initiate the SAML IdP.
		ossProcess.RegisterFunc(teleport.ComponentSAMLIdP, func() error {
			_, err := ossProcess.WaitForEvent(ossProcess.ExitContext(), service.ProxyWebServerReady)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(initSAMLIdP(ossProcess.ExitContext(), ossProcess.Config, webPlugin))
		})
	}

	return nil
}
