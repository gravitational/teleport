package main

import (
	"context"
	"io"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/tool/teleport/process"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/common"
)

func init() {
	metrics.RegisterPrometheusCollectors(metrics.BuildCollector())
}

func main() {
	app, executedCommand, config := common.Run(common.Options{
		Args:                  os.Args[1:],
		InitOnly:              true,
		EnableCloudAWSCredCmd: true,
	})
	startCmd := app.GetCommand("start")
	appStartCmd := app.GetCommand("app").GetCommand("start")
	dbStartCmd := app.GetCommand("db").GetCommand("start")
	cloudAWSCredCmd := app.GetCommand("cloud-aws-cred")
	switch executedCommand {
	case startCmd.FullCommand(), appStartCmd.FullCommand(), dbStartCmd.FullCommand():
		if err := service.Run(context.Background(), *config, process.NewTeleport); err != nil {
			utils.FatalError(err)
		}
	case cloudAWSCredCmd.FullCommand():
		if err := onCloudAWSCredCmd(config); err != nil {
			utils.FatalError(err)
		}
	}
}

// onCloudAWSCredCmd used with the credential process provider in AWS Shared Credential File
// to enable the cloud team to dynamically refresh AWS temporary credentials without restarting teleport.
func onCloudAWSCredCmd(cfg *servicecfg.Config) error {
	// cloudCredFile is defined in the cloud repository by the following code:
	// * https://github.com/gravitational/cloud/blob/2eae7dd12f81e2fd1b3e666ed1ae7b476345511c/pkg/teleportcontroller/deployments.go#L893
	// * https://github.com/gravitational/cloud/blob/2eae7dd12f81e2fd1b3e666ed1ae7b476345511c/pkg/teleportcontroller/iam_secret.go#L118
	const cloudCredFile = "/etc/aws-iam/credentials.json"

	licenseFile, err := licensefile.NewLicenseFile(cfg.Auth.LicenseFile)
	if err != nil {
		return trace.Wrap(err)
	}

	if !licenseFile.License.GetCloud().Value() {
		return trace.Errorf("This command can only be run from the cloud environment")
	}

	credFile, err := os.Open(cloudCredFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer credFile.Close()

	if _, err := io.Copy(os.Stdout, credFile); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
