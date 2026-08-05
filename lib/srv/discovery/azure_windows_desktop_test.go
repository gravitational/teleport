// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/azure/azuretest"
	"github.com/gravitational/teleport/lib/srv/server"
	"github.com/gravitational/teleport/lib/srv/server/installstatus"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	"k8s.io/client-go/kubernetes/fake"
)

// newWindowsTestServer builds a real *Server.
func newWindowsTestServer(t *testing.T, runClient *mockAzureRunCommandClient, fakeClock clockwork.Clock) *Server {
	t.Helper()

	testAuthServer, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir: t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testAuthServer.Close()) })

	_, err = testAuthServer.AuthServer.UpsertProxyServer(context.Background(), &types.ServerV2{
		Kind: types.KindProxy,
		Metadata: types.Metadata{
			Name: "proxy",
		},
		Spec: types.ServerSpecV2{
			PublicAddrs: []string{"proxy.example.com:443"},
		},
	})
	require.NoError(t, err)

	tlsServer, err := testAuthServer.NewTestTLSServer(authtest.WithBufconnListener())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tlsServer.Close()) })

	identity := authtest.TestServerID(types.RoleDiscovery, "hostID")
	authClient, err := tlsServer.NewClient(identity)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authClient.Close()) })

	logtest.InitLogger(func() bool { return true })

	initAzureClients := func(...azure.ClientsOption) (azure.Clients, error) {
		return &azuretest.Clients{
			AzureRunCommand: runClient,
		}, nil
	}

	s, err := New(authz.ContextWithUser(context.Background(), identity.I), &Config{
		initAzureClients: initAzureClients,
		ClusterFeatures:  func() proto.Features { return proto.Features{} },
		KubernetesClient: fake.NewClientset(),
		AccessPoint:      getDiscoveryAccessPoint(tlsServer.Auth(), authClient),
		Matchers:         Matchers{},
		Emitter:          &mockEmitter{},
		Log:              logtest.NewLogger(),
		DiscoveryGroup:   "dc001",
		PollInterval:     5 * time.Minute,
		clock:            fakeClock,
	})
	require.NoError(t, err)
	t.Cleanup(s.Stop)

	return s
}

func newWindowsVM(name, vmid, privateIP string) *azure.VirtualMachine {
	return &azure.VirtualMachine{
		ID:               "/subscriptions/testsub/resourceGroups/testrg/providers/Microsoft.Compute/virtualMachines/" + name,
		Name:             name,
		Subscription:     "testsub",
		ResourceGroup:    "testrg",
		Location:         "westcentralus",
		VMID:             vmid,
		Tags:             map[string]string{},
		PrimaryPrivateIP: privateIP,
	}
}

func newWindowsInstances(md server.AzureInstancesMetadata, vms ...*azure.VirtualMachine) *server.AzureInstances {
	return &server.AzureInstances{
		Metadata:  md,
		Instances: vms,
	}
}

func defaultWindowsMetadata(discoveryConfigName, integration string) server.AzureInstancesMetadata {
	return server.AzureInstancesMetadata{
		DiscoveryConfigName: discoveryConfigName,
		Integration:         integration,
		Region:              "westcentralus",
		SubscriptionID:      "testsub",
		ResourceGroup:       "testrg",
		MatcherType:         types.AzureMatcherWindowsVM,
		InstallerParams: &types.InstallerParams{
			WindowsScriptName: types.DefaultInstallerScriptNameWindowsAuthPackage,
		},
	}
}

// testHelper is the minimal subset of *testing.T (and *rapid.T, inside
// rapid.Check) that findDynamicWindowsDesktop needs.
type testHelper interface {
	require.TestingT
	Helper()
}

// findDynamicWindowsDesktop looks up a dynamic Windows desktop by name via
// the discovery service's AccessPoint.
func findDynamicWindowsDesktop(t testHelper, s *Server, name string) (types.DynamicWindowsDesktop, bool) {
	t.Helper()
	var pageToken string
	for {
		desktops, nextToken, err := s.AccessPoint.ListDynamicWindowsDesktops(context.Background(), 200, pageToken)
		require.NoError(t, err)
		for _, d := range desktops {
			if d.GetName() == name {
				return d, true
			}
		}
		if nextToken == "" {
			return nil, false
		}
		pageToken = nextToken
	}
}

func TestUpsertAzureWindowsDesktop(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, fakeClock)

	t.Run("no private IP", func(t *testing.T) {
		vm := newWindowsVM("no-ip-vm", "no-ip-vmid", "")
		err := s.createAzureWindowsDesktop(vm, fakeClock.Now())
		require.Error(t, err)
		require.True(t, errors.Is(err, errNoPrimaryPrivateIP), "expected errNoPrimaryPrivateIP, got %v", err)

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-no-ip-vmid")
		require.False(t, ok, "no desktop should have been created")
	})

	t.Run("creates desktop with expected fields", func(t *testing.T) {
		vm := newWindowsVM("good-vm", "good-vmid", "10.1.2.3")
		vm.Tags = map[string]string{"env": "prod"}

		syncTime := fakeClock.Now()
		require.NoError(t, s.createAzureWindowsDesktop(vm, syncTime))

		// Instancess are created with the name format "azure-windows-<vmid>".
		desktop, ok := findDynamicWindowsDesktop(t, s, "azure-windows-good-vmid")
		require.True(t, ok)

		require.Equal(t, net.JoinHostPort("10.1.2.3", "3389"), desktop.GetAddr())
		require.True(t, desktop.NonAD())
		require.True(t, syncTime.Add(s.PollInterval*5).Equal(desktop.Expiry()),
			"expected expiry %v, got %v", syncTime.Add(s.PollInterval*5), desktop.Expiry())

		labels := desktop.GetAllLabels()
		require.Equal(t, "prod", labels["env"], "VM tags should propagate to labels")

		// Internal labels used to match already-enrolled VMs.
		require.Equal(t, "testsub", labels[types.SubscriptionIDLabelInternal])
		require.Equal(t, "good-vmid", labels[types.VMIDLabelInternal])
		require.Equal(t, "testrg", labels[types.ResourceGroupLabelInternal])
		require.Equal(t, "westcentralus", labels[types.RegionLabelInternal])
		require.Equal(t, s.DiscoveryGroup, labels[types.TeleportInternalDiscoveryGroupName])

		// Public labels documented for users.
		require.Equal(t, types.CloudAzure, labels[types.CloudLabel])
		require.Equal(t, "testsub", labels[types.SubscriptionIDLabel])
		require.Equal(t, "good-vmid", labels[types.VMIDLabel])
		require.Equal(t, "testrg", labels[types.ResourceGroupLabel])
		require.Equal(t, "westcentralus", labels[types.RegionLabel])
	})

	t.Run("refresh updates existing desktop", func(t *testing.T) {
		vm := newWindowsVM("refresh-vm", "refresh-vmid", "10.1.2.4")
		firstSync := fakeClock.Now()
		require.NoError(t, s.createAzureWindowsDesktop(vm, firstSync))

		vm.PrimaryPrivateIP = "10.1.2.5"
		secondSync := firstSync.Add(time.Minute)
		require.NoError(t, s.createAzureWindowsDesktop(vm, secondSync))

		desktop, ok := findDynamicWindowsDesktop(t, s, "azure-windows-refresh-vmid")
		require.True(t, ok)
		require.Equal(t, net.JoinHostPort("10.1.2.5", "3389"), desktop.GetAddr())
		require.True(t, secondSync.Add(s.PollInterval*5).Equal(desktop.Expiry()),
			"expected expiry %v, got %v", secondSync.Add(s.PollInterval*5), desktop.Expiry())
	})
}

func TestRegisterDynamicWindowsDesktops(t *testing.T) {
	t.Parallel()

	fakeClock := clockwork.NewFakeClockAt(time.Now())
	s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, fakeClock)

	t.Run("failure result is rejected", func(t *testing.T) {
		vm := newWindowsVM("failed-vm", "failed-vmid", "10.1.2.3")
		result := server.AzureInstallResult{
			Instance:      vm,
			CommandResult: &azure.RunCommandResult{ExitCode: 1},
		}
		err := s.registerDynamicWindowsDesktops(result, fakeClock.Now())
		require.Error(t, err)

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-failed-vmid")
		require.False(t, ok, "no desktop should have been created for a failed result")
	})

	t.Run("successful result registers a desktop", func(t *testing.T) {
		vm := newWindowsVM("ok-vm", "ok-vmid", "10.1.2.3")
		result := server.AzureInstallResult{
			Instance: vm,
			CommandResult: &azure.RunCommandResult{
				ExecutionState: string(armcompute.ExecutionStateSucceeded),
				ExitCode:       0,
			},
		}
		require.NoError(t, s.registerDynamicWindowsDesktops(result, fakeClock.Now()))

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-ok-vmid")
		require.True(t, ok)
	})
}

func TestHandleAzureWindowsDesktops(t *testing.T) {
	t.Parallel()

	t.Run("no instances", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		s := newWindowsTestServer(t, &mockAzureRunCommandClient{}, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		emptyGroup := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""))
		status, err := s.handleAzureWindowsDesktops(emptyGroup, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Zero(t, status.found)
		require.Zero(t, status.enrolled)
		require.Zero(t, status.failed)
	})

	t.Run("new VM installs successfully", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		runClient := &mockAzureRunCommandClient{}
		s := newWindowsTestServer(t, runClient, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		vm := newWindowsVM("new-vm", "new-vmid", "10.2.0.1")
		group := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""), vm)

		status, err := s.handleAzureWindowsDesktops(group, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status.found)
		require.Equal(t, 1, status.enrolled)
		require.Equal(t, 0, status.failed)
		require.Equal(t, 1, runClient.getAttemptCount("new-vm"))

		desktop, ok := findDynamicWindowsDesktop(t, s, "azure-windows-new-vmid")
		require.True(t, ok)
		require.Equal(t, net.JoinHostPort("10.2.0.1", "3389"), desktop.GetAddr())
	})

	t.Run("already enrolled VM is refreshed, not reinstalled", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		runClient := &mockAzureRunCommandClient{}
		s := newWindowsTestServer(t, runClient, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		vm := newWindowsVM("enrolled-vm", "enrolled-vmid", "10.2.0.2")
		// Seed an existing desktop for this VM, as if a previous cycle
		// enrolled it already.
		require.NoError(t, s.createAzureWindowsDesktop(vm, fakeClock.Now()))

		group := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""), vm)
		status, err := s.handleAzureWindowsDesktops(group, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status.found)
		require.Equal(t, 1, status.enrolled)
		require.Equal(t, 0, status.failed)
		require.Equal(t, 0, runClient.getAttemptCount("enrolled-vm"), "already-enrolled VM should not trigger a reinstall")
	})

	t.Run("skip_installation registers desktop without running the installer", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		runClient := &mockAzureRunCommandClient{}
		s := newWindowsTestServer(t, runClient, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		vm := newWindowsVM("golden-vm", "golden-vmid", "10.2.0.3")
		md := defaultWindowsMetadata(NoDiscoveryConfig, "")
		md.InstallerParams = &types.InstallerParams{SkipInstallation: true}
		group := newWindowsInstances(md, vm)

		status, err := s.handleAzureWindowsDesktops(group, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status.found)
		require.Equal(t, 1, status.enrolled)
		require.Equal(t, 0, status.failed)
		require.Equal(t, 0, runClient.getAttemptCount("golden-vm"), "skip_installation should never call RunCommand")

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-golden-vmid")
		require.True(t, ok)
	})

	t.Run("missing private IP fails registration without touching RunCommand backoff for install", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		runClient := &mockAzureRunCommandClient{}
		s := newWindowsTestServer(t, runClient, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		vm := newWindowsVM("no-ip-vm", "no-ip-vmid", "")
		group := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""), vm)

		status, err := s.handleAzureWindowsDesktops(group, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status.found)
		require.Equal(t, 0, status.enrolled)
		require.Equal(t, 1, status.failed)
		// The install itself succeeded (RunCommand was called), but registration
		// would have failed.
		require.Equal(t, 1, runClient.getAttemptCount("no-ip-vm"))

		_, ok := findDynamicWindowsDesktop(t, s, "azure-windows-no-ip-vmid")
		require.False(t, ok)
	})

	t.Run("install API failure marks VM failed and backs off further attempts", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		runClient := &mockAzureRunCommandClient{}
		s := newWindowsTestServer(t, runClient, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		vm := newWindowsVM(azureApiErrorPrefix+"-vm", azureApiErrorPrefix+"-vmid", "10.2.0.4")
		group := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""), vm)

		status, err := s.handleAzureWindowsDesktops(group, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status.found)
		require.Equal(t, 0, status.enrolled)
		require.Equal(t, 1, status.failed)
		require.Equal(t, 1, runClient.getAttemptCount(azureApiErrorPrefix+"-vm"))

		// A second cycle immediately afterwards should be skipped by backoff,
		// not reattempted.
		group2 := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""), vm)
		status2, err := s.handleAzureWindowsDesktops(group2, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status2.failed)
		// Attempt count should stay at 1 from the previous cycle and not be
		// incremented (indicating a skipped attempt).
		require.Equal(t, 1, runClient.getAttemptCount(azureApiErrorPrefix+"-vm"), "VM should not be reinstalled during the backoff window")
	})

	t.Run("install script failure is classified from exit code", func(t *testing.T) {
		fakeClock := clockwork.NewFakeClockAt(time.Now())
		runClient := &mockAzureRunCommandClient{}
		s := newWindowsTestServer(t, runClient, fakeClock)
		backoff, err := newInstallerBackoff(s.PollInterval*2, retryutils.SeventhJitter)
		require.NoError(t, err)

		vm := newWindowsVM(azureInstallErrorPrefix+"-vm", azureInstallErrorPrefix+"-vmid", "10.2.0.5")
		group := newWindowsInstances(defaultWindowsMetadata(NoDiscoveryConfig, ""), vm)

		status, err := s.handleAzureWindowsDesktops(group, &azureVMTasks{}, backoff, semaphore.NewWeighted(10))
		require.NoError(t, err)
		require.Equal(t, 1, status.failed)
		require.Equal(t, 0, status.enrolled)

		_, ok := findDynamicWindowsDesktop(t, s, "bad-install-vmid")
		require.False(t, ok)
	})
}

func TestClassifyAzureWindowsAuthPackageInstallResultIssue(t *testing.T) {
	t.Parallel()

	vm := newWindowsVM("vm", "vmid", "10.0.0.1")

	tests := []struct {
		name      string
		exitCode  installstatus.ExitCode
		wantIssue string
	}{
		{"disk space", installstatus.WindowsInsufficientDiskSpace, "azure-vm-windows-auth-package-insufficient-disk-space"},
		{"unsupported version", installstatus.UnsupportedWindowsVersion, "azure-vm-windows-auth-package-unsupported-windows-version"},
		{"domain joined", installstatus.WindowsMachineDomainJoined, "azure-vm-windows-auth-package-machine-domain-joined"},
		{"download failure", installstatus.WindowsInstallerDownloadFailure, "azure-vm-windows-auth-package-download-failure"},
		{"execution failure", installstatus.WindowsInstallerExecutionFailure, "azure-vm-windows-auth-package-execution-failure"},
		{"staging dir unsafe", installstatus.WindowsInstallerStagingDirUnsafe, "azure-vm-windows-auth-package-staging-dir-unsafe"},
		{"checksum mismatch", installstatus.WindowsInstallerChecksumMismatch, "azure-vm-windows-auth-package-checksum-mismatch"},
		{"unrecognized code falls back to generic", installstatus.ExitCode(999), "azure-vm-enrollment-error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := server.AzureInstallResult{
				Instance:      vm,
				CommandResult: &azure.RunCommandResult{ExitCode: int32(tc.exitCode)},
			}
			require.Equal(t, tc.wantIssue, classifyAzureWindowsAuthPackageInstallResultIssue(result))
		})
	}

	t.Run("API error falls back to shared classifier", func(t *testing.T) {
		result := server.AzureInstallResult{
			Instance: vm,
			APIError: nil,
		}
		require.Empty(t, classifyAzureWindowsAuthPackageInstallResultIssue(result))
	})
}

func TestClassifyAzureWindowsDesktopRegistrationError(t *testing.T) {
	t.Parallel()

	require.Empty(t, classifyAzureWindowsDesktopRegistrationError(nil))
	require.Equal(t,
		"azure-vm-windows-auth-package-no-private-ip",
		classifyAzureWindowsDesktopRegistrationError(errNoPrimaryPrivateIP),
	)
	require.Equal(t,
		"azure-vm-windows-auth-package-no-private-ip",
		classifyAzureWindowsDesktopRegistrationError(trace.Wrap(errNoPrimaryPrivateIP, "wrapped")),
		"errors.Is should match errNoPrimaryPrivateIP through trace.Wrap",
	)
	require.Equal(t, "azure-vm-enrollment-error", classifyAzureWindowsDesktopRegistrationError(errors.New("some other backend error")))
}

func newDynamicDesktop(name, origin, discoveryGroup string) (*types.DynamicWindowsDesktopV1, error) {
	labels := map[string]string{
		types.OriginLabel:                        origin,
		types.TeleportInternalDiscoveryGroupName: discoveryGroup,
	}

	return types.NewDynamicWindowsDesktopV1(name, labels, types.DynamicWindowsDesktopSpecV1{
		Addr:  net.JoinHostPort("10.0.0.1", "3389"),
		NonAD: true,
	})
}

func TestCheckDesktopIsDiscoveryManaged(t *testing.T) {
	t.Parallel()

	const discoveryGroup = "group1"

	for _, tc := range []struct {
		name           string
		origin         string
		discoveryGroup string
		errCheck       require.ErrorAssertionFunc
	}{
		{
			name:           "matching origin and discovery group",
			origin:         types.OriginCloud,
			discoveryGroup: discoveryGroup,
			errCheck:       require.NoError,
		},
		{
			// An agent with a discovery group claims desktops discovered by an
			// agent that had none configured.
			name:           "empty discovery group is claimed",
			origin:         types.OriginCloud,
			discoveryGroup: "",
			errCheck:       require.NoError,
		},
		{
			name:           "different discovery group",
			origin:         types.OriginCloud,
			discoveryGroup: "group2",
			errCheck:       require.Error,
		},
		{
			// A desktop created by hand or by another subsystem must never be
			// overwritten by auto-discovery.
			name:           "non-cloud origin",
			origin:         types.OriginDynamic,
			discoveryGroup: discoveryGroup,
			errCheck:       require.Error,
		},
		{
			name:           "missing origin",
			origin:         "",
			discoveryGroup: discoveryGroup,
			errCheck:       require.Error,
		},
		{
			name:           "non-cloud origin and different discovery group",
			origin:         types.OriginDynamic,
			discoveryGroup: "group2",
			errCheck:       require.Error,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			desktop, err := newDynamicDesktop("desktop", tc.origin, tc.discoveryGroup)
			require.NoError(t, err)
			tc.errCheck(t, checkDesktopIsDiscoveryManaged(desktop, discoveryGroup))
		})
	}
}
