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
