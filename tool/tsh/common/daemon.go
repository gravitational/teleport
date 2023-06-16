/*
Copyright 2021 Gravitational, Inc.

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
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/lib/teleterm"
	"github.com/gravitational/teleport/lib/utils"
)

// onDaemonStart implements the "tsh daemon start" command.
func onDaemonStart(cf *CLIConf) error {
	homeDir := profile.FullProfilePath(cf.HomePath)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cf.Debug {
		utils.InitLogger(utils.LoggingForDaemon, logrus.DebugLevel)
	} else {
		utils.InitLogger(utils.LoggingForDaemon, logrus.InfoLevel)
	}

	err := teleterm.Serve(ctx, teleterm.Config{
		HomeDir:            homeDir,
		CertsDir:           cf.DaemonCertsDir,
		Addr:               cf.DaemonAddr,
		InsecureSkipVerify: cf.InsecureSkipVerify,
		PrehogAddr:         cf.DaemonPrehogAddr,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
