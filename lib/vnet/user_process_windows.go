// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package vnet

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/user"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/sys/windows"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
)

// runPlatformUserProcess launches a Windows service in the background that will
// handle all networking and OS configuration. The user process exposes a gRPC
// interface that the admin process uses to query application names and get user
// certificates for apps. It returns a [ProcessManager] which controls the
// lifecycle of both the user and admin processes.
func runPlatformUserProcess(ctx context.Context, config *UserProcessConfig) (pm *ProcessManager, nsi NetworkStackInfo, err error) {
	// Make sure to close the process manager if returning a non-nil error.
	defer func() {
		if pm != nil && err != nil {
			pm.Close()
		}
	}()

	// On Windows the interface name is a constant.
	nsi.IfaceName = tunInterfaceName

	ipcCreds, err := newIPCCredentials()
	if err != nil {
		return nil, nsi, trace.Wrap(err, "creating credentials for IPC")
	}
	serverTLSConfig, err := ipcCreds.server.serverTLSConfig()
	if err != nil {
		return nil, nsi, trace.Wrap(err, "generating gRPC server TLS config")
	}

	u, err := user.Current()
	if err != nil {
		return nil, nsi, trace.Wrap(err, "getting current OS user")
	}
	// Uid is documented to be the user's SID on Windows.
	userSID := u.Uid

	credDir, err := os.MkdirTemp("", "vnet_service_certs")
	if err != nil {
		return nil, nsi, trace.Wrap(err, "creating temp dir for service certs")
	}
	if err := secureCredDir(credDir, userSID); err != nil {
		return nil, nsi, trace.Wrap(err, "applying permissions to service credential dir")
	}
	if err := ipcCreds.client.write(credDir); err != nil {
		return nil, nsi, trace.Wrap(err, "writing service IPC credentials")
	}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nsi, trace.Wrap(err, "listening on tcp socket")
	}
	// grpcServer.Serve takes ownership of (and closes) the listener.
	grpcServer := grpc.NewServer(
		grpc.Creds(grpccredentials.NewTLS(serverTLSConfig)),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
	)
	clock := clockwork.NewRealClock()
	appProvider := newLocalAppProvider(config.ClientApplication, clock)
	svc := newClientApplicationService(appProvider)
	vnetv1.RegisterClientApplicationServiceServer(grpcServer, svc)

	pm, processCtx := newProcessManager()
	pm.AddCriticalBackgroundTask("admin process", func() error {
		log.InfoContext(processCtx, "Starting Windows service")
		defer func() {
			// Delete service credentials after the service terminates.
			if ipcCreds.client.remove(credDir); err != nil {
				log.ErrorContext(ctx, "Failed to remove service credential files", "error", err)
			}
			if err := os.RemoveAll(credDir); err != nil {
				log.ErrorContext(ctx, "Failed to remove service credential directory", "error", err)
			}
		}()
		return trace.Wrap(runService(processCtx, &windowsAdminProcessConfig{
			clientApplicationServiceAddr: listener.Addr().String(),
			serviceCredentialPath:        credDir,
			userSID:                      userSID,
		}))
	})
	pm.AddCriticalBackgroundTask("gRPC service", func() error {
		log.InfoContext(processCtx, "Starting gRPC service",
			"addr", listener.Addr().String())
		return trace.Wrap(grpcServer.Serve(listener),
			"serving VNet user process gRPC service")
	})
	pm.AddCriticalBackgroundTask("gRPC server closer", func() error {
		// grpcServer.Serve does not stop on its own when processCtx is done, so
		// this task waits for processCtx and then explicitly stops grpcServer.
		<-processCtx.Done()
		grpcServer.GracefulStop()
		return nil
	})
	return pm, nsi, nil
}

// secureCredDir sets ACLs so that the current user can write credentials files
// to dir but cannot read them. Only the LocalSystem account is allowed to read
// and delete the files.
func secureCredDir(dir string, userSID string) error {
	// S-1-5-18 is the well-known SID (security identifier) for the LocalSystem
	// service account, which the VNet windows service runs as.
	// * https://learn.microsoft.com/en-us/windows/win32/secauthz/well-known-sids
	const serviceSID = "S-1-5-18"
	// O: Set Owner as current user (Windows won't allow LocalSystem to be the owner)
	// G: Set Primary Group as current user
	// D: Discretionary ACL
	//   Grant GA (Generic All) to LocalSystem
	//   Grant GWSD (Generic Write, Standard Delete) to the current user so we can add files and later delete them
	//   Deny GR (Generic Read) to WD (Everyone*) so other processes can't read the credentials
	//   * This doesn't seems to deny access to the LocalSystem account, but all users are denied read access
	//   * https://learn.microsoft.com/en-us/windows/win32/secauthz/sid-strings?utm_source=chatgpt.com
	//   OICI (Object Inherit, Container Inherit) propagates permissions to files/folders
	sdString := fmt.Sprintf("O:%[1]sG:%[1]sD:"+
		"(A;OICI;GA;;;%[2]s)"+
		"(A;OICI;GWSD;;;%[1]s)"+
		"(D;OICI;GR;;;WD)",
		userSID, serviceSID,
	)
	if err := applySecurityDescriptor(sdString, dir); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func applySecurityDescriptor(sdString, path string) error {
	sd, err := windows.SecurityDescriptorFromString(sdString)
	if err != nil {
		return trace.Wrap(err, "parsing security descriptor string %s", sdString)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return trace.Wrap(err, "getting owner from security descriptor")
	}
	group, _, err := sd.Group()
	if err != nil {
		return trace.Wrap(err, "getting group from security descriptor")
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return trace.Wrap(err, "getting DACL from security descriptor")
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
		owner,
		group,
		dacl,
		nil, // SACL
	); err != nil {
		return trace.Wrap(err, "setting security info on service credential file %s", path)
	}
	return nil
}
