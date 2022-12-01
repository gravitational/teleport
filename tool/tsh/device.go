// Copyright 2022 Gravitational, Inc
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
	"fmt"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/enroll"
)

type deviceCommand struct {
	enroll *deviceEnrollCommand
}

func newDeviceCommand(app *kingpin.Application) *deviceCommand {
	root := &deviceCommand{
		enroll: &deviceEnrollCommand{},
	}

	// "tsh device" command.
	parentCmd := app.Command(
		"device", "Manage your device. Requires Teleport Enterprise.")

	// "tsh device enroll" command.
	root.enroll.CmdClause = parentCmd.Command(
		"enroll", "Enroll your device as a trusted device. Requires Teleport Enteprise")
	root.enroll.Flag("token", "Device enrollment token").
		Required().
		StringVar(&root.enroll.token)

	return root
}

type deviceEnrollCommand struct {
	*kingpin.CmdClause

	token string
}

func (c *deviceEnrollCommand) run(cf *CLIConf) error {
	teleportClient, err := makeClient(cf, true /* useProfileLogin */)
	if err != nil {
		return trace.Wrap(err)
	}

	var dev *devicepb.Device
	ctx := cf.Context
	if err := client.RetryWithRelogin(ctx, teleportClient, func() error {
		proxyClient, err := teleportClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err := proxyClient.ConnectToRootCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		devices := authClient.DevicesClient()
		dev, err = enroll.RunCeremony(ctx, devices, c.token)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"Device %q/%v enrolled\n",
		dev.AssetTag, devicetrust.FriendlyOSType(dev.OsType),
	)
	return nil
}
