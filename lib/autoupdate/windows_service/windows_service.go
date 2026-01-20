package windows_service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/vnet"
)

const serviceName = "TeleportUpdateService"
const ServiceCommand = "update-service"
const serviceAccessFlags = windows.SERVICE_START | windows.SERVICE_QUERY_STATUS
const pipePath = `\\.\pipe\UpdateServicePipe`
const maxMetadataSize = 1 << 20
const maxUpdatePayloadSize = 1_000_000_000
const pipeSecurityDescriptor = "D:(A;;GA;;;SY)(A;;GA;;;BA)(A;;GRGW;;;AU)(D;;GRGW;;;WD)"

var log = logutils.NewPackageLogger(teleport.ComponentKey, "update-service")

const eventSource = "updater-service"

func InstallService(ctx context.Context) (err error) {
	return trace.Wrap(vnet.InstallService(ctx, &vnet.ServiceConfig{
		Name:              serviceName,
		Command:           ServiceCommand,
		EventSourceName:   eventSource,
		AccessPermissions: serviceAccessFlags,
	}))
}

func ServiceMain() error {
	err := vnet.RunWindowsServiceMain(serviceName, &winServ{}, eventSource)
	return trace.Wrap(err)
}

type winServ struct{}

func (*winServ) Run(ctx context.Context, _ []string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	meta, secureUpdatePath, err := receiveUpdateFromClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	//
	//if err = ensureIsUpgrade(meta.Version); err != nil {
	//	return trace.Wrap(err)
	//}

	hash, err := downloadHash(ctx, meta.Version)
	if err != nil {
		return trace.Wrap(err, "downloading update hash")
	}

	if err = verifyUpdateHash(secureUpdatePath, hash); err != nil {
		return trace.Wrap(err, "verifying update hash")
	}

	if err = verifySignature(secureUpdatePath); err != nil {
		return trace.Wrap(err, "verifying update hash")
	}

	return trace.Wrap(runInstaller(secureUpdatePath, meta), "running admin process")
}

func receiveUpdateFromClient(ctx context.Context) (*UpdateMetadata, string, error) {
	conn, err := waitForClient(ctx)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	defer conn.Close()

	dir, err := secureUpdateDir()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	meta, updatePath, err := readUpdate(conn, dir)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return meta, updatePath, nil
}

func verifySignature(updatePath string) error {
	servicePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	serviceSignature, err := readSignature(servicePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if serviceSignature.Status != "Valid" || serviceSignature.Subject == "" {
		log.Info("Service binary not signed; skipping installer signature verification")
		return nil
	}

	updateSignature, err := readSignature(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if updateSignature.Status != "Valid" {
		return trace.BadParameter("installer signature is not valid")
	}
	if updateSignature.Subject == "" {
		return trace.BadParameter("installer signature subject is empty")
	}
	if updateSignature.Subject != serviceSignature.Subject {
		return trace.BadParameter("installer signature subject does not match service signature")
	}
	return nil
}

func runInstaller(updatePath string, meta *UpdateMetadata) error {
	args := []string{"--updated", "/S"}
	if meta.ForceRun {
		args = append(args, "--force-run")
	}
	cmd := exec.Command(updatePath, args...)

	log.Info("Running command", "command", cmd.String())

	// Use Start() instead of Run().
	// Start() returns immediately after the process is launched.
	err := cmd.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	// Important: Release the handle to the process so the parent
	// doesn't keep a reference to the child in its process table.
	if cmd.Process != nil {
		log.Info("Releasing resources")
		err = cmd.Process.Release()
	}

	return trace.Wrap(err)
}

func downloadHash(ctx context.Context, version string) ([]byte, error) {
	url := fmt.Sprintf("https://cdn.teleport.dev/Teleport Connect Setup-%s.exe.sha256", version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []byte{}, trace.BadParameter("update hash request failed with status %s", resp.Status)
	}

	var buf bytes.Buffer
	_, err = io.CopyN(&buf, resp.Body, sha256.Size*2) // SHA bytes to hex
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}
	expected, err := hex.DecodeString(buf.String())
	if err != nil {
		return []byte{}, trace.Wrap(err)
	}
	return expected, trace.Wrap(err)
}

func verifyUpdateHash(updatePath string, expectedHash []byte) error {
	file, err := os.Open(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return trace.Wrap(err)
	}
	actual := hasher.Sum(nil)
	if !bytes.Equal(actual, expectedHash) {
		return trace.BadParameter("hash of archive does not match downloaded archive")
	}
	return nil
}

func ensureIsUpgrade(updateVersion string) error {
	updateSemver, err := semver.NewVersion(updateVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	current := teleport.SemVer()
	if current == nil {
		return trace.BadParameter("current version is not available")
	}
	if updateSemver.Compare(*current) <= 0 {
		return trace.BadParameter("update version %s is not newer than current version %s", updateSemver, current)
	}
	return nil
}

func waitForClient(ctx context.Context) (net.Conn, error) {
	l, err := winio.ListenPipe(pipePath, &winio.PipeConfig{
		SecurityDescriptor: pipeSecurityDescriptor,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = l.Close()
		case <-done:
		}
	}()

	// 2. Accept exactly ONE connection
	conn, err := l.Accept()
	close(done)
	if err != nil {
		if ctx.Err() != nil {
			return nil, trace.Wrap(ctx.Err())
		}
		return nil, trace.Wrap(err)
	}

	// 3. Stop Listening immediately!
	// This removes the pipe name from the system.
	// Any 2nd client trying to connect now will get "File Not Found".
	// The existing 'conn' stays alive.
	err = l.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return conn, nil
}

type UpdateMetadata struct {
	ForceRun bool   `json:"force_run"`
	Version  string `json:"version"`
	// You can add file hash here for extra security
}

func readUpdate(conn io.ReadWriteCloser, destinationDir string) (*UpdateMetadata, string, error) {
	// 1. Read Header Length
	var jsonLen uint32
	if err := binary.Read(conn, binary.LittleEndian, &jsonLen); err != nil {
		return nil, "", trace.Wrap(err)
	}
	if jsonLen > maxMetadataSize {
		return nil, "", trace.BadParameter("metadata payload too large")
	}

	// 2. Read Metadata JSON
	// Limit the reader to exactly jsonLen bytes
	metaBuf := make([]byte, jsonLen)
	if _, err := io.ReadFull(conn, metaBuf); err != nil {
		return nil, "", trace.Wrap(err)
	}

	var meta UpdateMetadata
	if err := json.Unmarshal(metaBuf, &meta); err != nil {
		return nil, "", trace.Wrap(err)
	}

	outFilePath := filepath.Join(destinationDir, "teleport-update.exe")

	// 3. Create/Open the actual File
	outFile, err := os.OpenFile(outFilePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	// Defer closing the file. Note: We might want to close explicitly before executing.
	defer outFile.Close()

	// io.Copy reads outFilePath EOF (connection closed), but cap the payload size.
	limited := &io.LimitedReader{R: conn, N: maxUpdatePayloadSize + 1}
	n, err := io.Copy(outFile, limited)
	if err != nil {
		_ = outFile.Close()
		_ = os.Remove(outFilePath)
		return nil, "", trace.Wrap(err)
	}
	if n > maxUpdatePayloadSize {
		_ = outFile.Close()
		_ = os.Remove(outFilePath)
		return nil, "", trace.BadParameter("update payload exceeds max size")
	}

	return &meta, outFilePath, nil
}

func secureUpdateDir() (string, error) {
	programData, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", trace.Wrap(err, "reading ProgramData path")
	}

	// 1. Ensure Parent Exists (The container folder)
	// ProgramData is public, so we don't care about the parent's ACLs,
	// only that our specific leaf directory is locked down.
	parentDir := filepath.Join(programData, "TeleportConnectUpdates")
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return "", trace.Wrap(err, "creating parent directory")
	}

	// 2. Generate Random UUID-like Name
	// We use crypto/rand to ensure it is unpredictable.
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return "", trace.Wrap(err, "generating random ID")
	}
	uniqueID := hex.EncodeToString(randBytes)

	targetDir := filepath.Join(parentDir, uniqueID)

	// 3. Prepare Security Attributes (Admin/System ONLY)
	// D:   = DACL
	// (A;OICI;GA;;;SY) = Allow System Full Access
	// (A;OICI;GA;;;BA) = Allow Admins Full Access
	// Implicitly DENY everyone else.
	const sddl = "D:(A;OICI;GA;;;SY)(A;OICI;GA;;;BA)"

	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return "", trace.Wrap(err, "creating security descriptor")
	}

	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}

	// 4. Atomic Secure Creation
	// We use CreateDirectory directly.
	// If this UUID somehow exists (statistically impossible), we fail safely.
	namePtr, _ := windows.UTF16PtrFromString(targetDir)
	err = windows.CreateDirectory(namePtr, sa)
	if err != nil {
		return "", trace.Wrap(err, "creating secure directory")
	}

	return targetDir, nil
}

type signature struct {
	Status      string `json:"Status"`      // Valid, NotSigned, etc.
	Subject     string `json:"Subject"`     // "CN=My Company, O=..."
	FileVersion string `json:"FileVersion"` // "1.2.3.4"
}

func readSignature(path string) (*signature, error) {
	// PowerShell Script to run
	// 1. Get signature
	// 2. Get Version Info
	// 3. Output as compressed JSON
	psScript := `
    $path = $args[0]
    $sig = Get-AuthenticodeSignature -LiteralPath $path
    $ver = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($path)

    $obj = @{
       Status = $sig.Status.ToString()
       Subject = if ($sig.SignerCertificate) { $sig.SignerCertificate.Subject } else { "" }
       FileVersion = $ver.FileVersion
    }
    $obj | ConvertTo-Json -Compress
`

	// Wrap in a command
	// -NoProfile: Speeds up load time
	// -NonInteractive: Prevents prompts
	// Pass the path as an argument to the script block.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript, path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("powershell error: %v, output: %s", err, string(output))
	}

	// Parse JSON
	var info signature
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse json: %v | raw: %s", err, string(output))
	}

	return &info, nil
}
