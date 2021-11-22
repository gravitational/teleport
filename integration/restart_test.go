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

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/service"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestLostConnectionToAuthCausesReload tests that a lost connection to the auth server
// will eventually restart a node
func TestLostConnectionToAuthCausesReload(t *testing.T) {
	// Because testing that the node does a full restart is a bit tricky when
	// running a cluster from inside a test runner (i.e. we don't want to
	// SIGTERM the test runner), we will watch for the node emitting a
	// `TeleportReload` even instead. In a proper Teleport instance, this
	// event would be picked up at the Supervisor level and would eventually
	// cause the instance to gracefully restart.

	require := require.New(t)
	log := log.StandardLogger()

	log.Info(">>> Entering Test")

	// InsecureDevMode needed for SSH connections
	// TODO(tcsc): surface this as per-server config (see also issue #8913)
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// GIVEN  a cluster with a running auth+proxy instance....
	log.Info(">>> Creating cluster")
	keygen := testauthority.New()
	privateKey, publicKey, err := keygen.GenerateKeyPair("")
	require.NoError(err)
	auth := NewInstance(InstanceConfig{
		ClusterName: "test-tunnel-collapse",
		HostID:      "auth",
		Priv:        privateKey,
		Pub:         publicKey,
		Ports:       standardPortSetup(),
		log:         log,
	})

	log.Info(">>> Creating auth-proxy...")
	authConf := service.MakeDefaultConfig()
	authConf.Hostname = Host
	authConf.Auth.Enabled = true
	authConf.Proxy.Enabled = true
	authConf.SSH.Enabled = false
	authConf.Proxy.DisableWebInterface = true
	authConf.Proxy.DisableDatabaseProxy = true
	require.NoError(auth.CreateEx(t, nil, authConf))
	t.Cleanup(func() { require.NoError(auth.StopAll()) })

	log.Info(">>> Start auth-proxy...")
	require.NoError(auth.Start())

	// ... and an SSH node connected via a reverse tunnel configured to
	// reload after only a few failed connection attempts per minute
	log.Info(">>> Creating and starting node...")
	nodeCfg := service.MakeDefaultConfig()
	nodeCfg.Hostname = Host
	nodeCfg.SSH.Enabled = true
	nodeCfg.RotationConnectionInterval = 1 * time.Second
	nodeCfg.RestartThreshold = service.Rate{Amount: 3, Time: 1 * time.Minute}
	node, err := auth.StartReverseTunnelNode(nodeCfg)
	require.NoError(err)

	// WHEN I stop the auth node (and, by implication, disrupt the ssh node's
	// connection to it)
	log.Info(">>> Stopping auth node")
	auth.StopAuth(false)

	// EXPECT THAT the ssh node will eventually issue a reload request
	log.Info(">>> Waiting for node restart request.")
	waitCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	eventCh := make(chan service.Event)
	node.WaitForEvent(waitCtx, service.TeleportReloadEvent, eventCh)
	select {
	case e := <-eventCh:
		log.Infof(">>> Received Reload event: %v. Test passed.", e)

	case <-waitCtx.Done():
		require.FailNow("Timed out", "Timed out waiting for reload event")
	}

	log.Info(">>> TEST COMPLETE")
}
