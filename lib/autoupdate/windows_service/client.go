package windows_service

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// RunServiceAndInstallFromClient is called by the client.
func RunServiceAndInstallFromClient(path string, forceRun bool, version string) error {
	err := ensureServiceRunning()
	if err != nil {
		return trace.Wrap(err)
	}

	// STEP 2: Connect to Pipe (with Retries)
	// Even if the service is "Running", the Pipe listener might take another 100ms to open.

	fmt.Println("Waiting for pipe...")
	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	fmt.Println("Connected to Service!")

	// STEP 3: Send Data (Same as before)
	meta := UpdateMetadata{ForceRun: forceRun, Version: version}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return trace.Wrap(err)
	}

	// Send Length
	err = binary.Write(conn, binary.LittleEndian, uint32(len(metaBytes)))
	if err != nil {
		return trace.Wrap(err)

	}
	// Send JSON
	_, err = conn.Write(metaBytes)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("Will open path: %s\n", path)
	// Stream File
	file, err := os.Open(path)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	_, err = io.Copy(conn, file)
	return trace.Wrap(err)
}

func ensureServiceRunning() error {
	// 1. Connect to Service Control Manager
	// Avoid [mgr.Connect] because it requests elevated permissions.
	scManager, err := windows.OpenSCManager(nil /*machine*/, nil /*database*/, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return trace.Wrap(err, "opening Windows service manager")
	}
	defer windows.CloseServiceHandle(scManager)
	serviceNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return trace.Wrap(err, "converting service name to UTF16")
	}
	serviceHandle, err := windows.OpenService(scManager, serviceNamePtr, serviceAccessFlags)
	if err != nil {
		return trace.Wrap(err, "opening Windows service %v", serviceName)
	}
	service := &mgr.Service{
		Name:   serviceName,
		Handle: serviceHandle,
	}

	// 3. Check Status
	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("could not query status: %v", err)
	}

	if status.State == svc.Running {
		return nil // Already running
	}

	fmt.Println("Service is stopped. Attempting to start...")

	// 4. Start the Service
	if err := service.Start(ServiceCommand); err != nil {
		return trace.Wrap(err, "starting Windows service %s", serviceName)
	}

	// 5. Wait for it to actually run (Polling)
	// We wait up to 10 seconds for the state to become 'Running'
	for i := 0; i < 20; i++ {
		status, err = service.Query()
		if err == nil && status.State == svc.Running {
			fmt.Println("Service started successfully.")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for service to start")
}
