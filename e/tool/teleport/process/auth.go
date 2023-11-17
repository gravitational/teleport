package process

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/cloud"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
)

// extendAuthServer extends the auth server with enterprise specific features.
func extendAuthServer(process *service.TeleportProcess, licenseFile *licensefile.LicenseFile, authPlugin *auth.Plugin) (service.Process, error) {
	process.Config.Log.Info("Starting enterprise auth services")
	cleanup, err := auth.StartServices(process.ExitContext(), authPlugin)
	if err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}
	process.Config.Log.Info("Finished starting enterprise auth services")

	process.OnExit("enterprise.auth.services.stop", func(_ interface{}) {
		process.Config.Log.Info("Cleaning up enterprise auth services.")
		cleanup()
		process.Config.Log.Info("Finished cleaning up enterprise auth services.")
	})

	// Initialize teleport cloud process
	if modules.GetModules().Features().Cloud {
		cloudProcess, err := cloud.NewTeleport(cloud.Config{
			AuthPlugin:  authPlugin,
			OSSProcess:  process,
			LicenseFile: licenseFile,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return cloudProcess, nil
	}

	// Initialize self-hosted enterprise teleport process
	proProcess, err := pro.NewTeleport(pro.Config{
		AuthPlugin:  authPlugin,
		OSSProcess:  process,
		LicenseFile: licenseFile,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return proProcess, nil

}
