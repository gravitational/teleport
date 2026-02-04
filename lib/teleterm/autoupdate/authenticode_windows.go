package autoupdate

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// CMSG_SIGNER_CERT_INFO_PARAM retrieves a CERT_INFO structure directly.
	// This avoids defining cmsgSignerInfo, certInfo, and all their substructs.
	cmsgSignerCertInfoParam = 7
)

var (
	crypt32              = windows.NewLazySystemDLL("crypt32.dll")
	procCryptMsgGetParam = crypt32.NewProc("CryptMsgGetParam")
	procCryptMsgClose    = crypt32.NewProc("CryptMsgClose")
)

// VerifySignature checks if the update is signed by the same entity as the running service.
func VerifySignature(updatePath string) error {
	servicePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// 1. Who signed the currently running service?
	serviceSubject, err := getSignerSubject(servicePath)
	if err != nil {
		return fmt.Errorf("checking service signature: %w", err)
	}

	// If current service isn't signed, we skip verification (dev mode/unsigned builds)
	if serviceSubject == "" {
		return nil
	}

	// 2. Who signed the new update?
	updateSubject, err := getSignerSubject(updatePath)
	if err != nil {
		return fmt.Errorf("checking update signature: %w", err)
	}

	// 3. Do they match?
	if updateSubject == "" {
		return errors.New("update is not signed, but service is")
	}
	if updateSubject != serviceSubject {
		return fmt.Errorf("signer mismatch: expected %q, got %q", serviceSubject, updateSubject)
	}

	return nil
}

func getSignerSubject(path string) (string, error) {
	// Step 1: Verify the file is trusted by the OS (Integrity Check)
	if err := verifyAuthenticode(path); err != nil {
		if isNoSignature(err) {
			return "", nil // Not signed
		}
		return "", err
	}

	// Step 2: Open the file as a Crypto Object
	path16, _ := windows.UTF16PtrFromString(path)
	var (
		msgHandle, storeHandle windows.Handle
		encoding               uint32
	)

	err := windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(path16),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0,
		&encoding,
		nil, nil,
		&storeHandle,
		&msgHandle,
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("CryptQueryObject: %w", err)
	}
	defer windows.CertCloseStore(storeHandle, 0)
	defer cryptMsgClose(msgHandle)

	// Step 3: Get the CERT_INFO blob directly (The Magic Trick)
	// We use param 7 (CMSG_SIGNER_CERT_INFO_PARAM) instead of 6.
	// This gives us a blob we can pass directly to CertFindCertificateInStore.
	certInfoBlob, err := getCryptMsgParam(msgHandle, cmsgSignerCertInfoParam)
	if err != nil {
		return "", fmt.Errorf("failed to get signer cert info: %w", err)
	}

	// Step 4: Find the matching certificate in the store
	certCtx, err := windows.CertFindCertificateInStore(
		storeHandle,
		encoding,
		0,
		windows.CERT_FIND_SUBJECT_CERT,
		unsafe.Pointer(&certInfoBlob[0]), // Pass the opaque blob
		nil,
	)
	if err != nil {
		return "", fmt.Errorf("signer certificate not found in store: %w", err)
	}
	defer windows.CertFreeCertificateContext(certCtx)

	// Step 5: Read the Subject Name (CN/O)
	// 0 = CERT_NAME_SIMPLE_DISPLAY_TYPE (Common Name or Email)
	// You can change 0 to windows.CERT_NAME_RDN_TYPE to get the full DN.
	lenName := windows.CertGetNameString(certCtx, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, nil, 0)
	if lenName <= 1 {
		return "", nil
	}
	buf := make([]uint16, lenName)
	windows.CertGetNameString(certCtx, windows.CERT_NAME_SIMPLE_DISPLAY_TYPE, 0, nil, &buf[0], lenName)

	return windows.UTF16ToString(buf), nil
}

func verifyAuthenticode(path string) error {
	path16, _ := windows.UTF16PtrFromString(path)
	fileInfo := windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: path16,
	}
	data := windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_WHOLECHAIN,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fileInfo),
	}

	// Verify
	guid := windows.WINTRUST_ACTION_GENERIC_VERIFY_V2
	err := windows.WinVerifyTrustEx(windows.InvalidHWND, &guid, &data)

	// Close State (Crucial!)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &guid, &data)

	return err
}

// Helper to handle the C-style size-then-data pattern
func getCryptMsgParam(handle windows.Handle, paramType uint32) ([]byte, error) {
	var size uint32
	// First call to get size
	r1, _, _ := syscall.SyscallN(procCryptMsgGetParam.Addr(), uintptr(handle), uintptr(paramType), 0, 0, uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return nil, fmt.Errorf("failed to get param size")
	}

	buf := make([]byte, size)
	// Second call to get data
	r1, _, _ = syscall.SyscallN(procCryptMsgGetParam.Addr(), uintptr(handle), uintptr(paramType), 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r1 == 0 {
		return nil, fmt.Errorf("failed to get param data")
	}
	return buf, nil
}

func cryptMsgClose(handle windows.Handle) {
	syscall.SyscallN(procCryptMsgClose.Addr(), uintptr(handle))
}

func isNoSignature(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.Errno(windows.TRUST_E_NOSIGNATURE) ||
			errno == syscall.Errno(windows.TRUST_E_SUBJECT_FORM_UNKNOWN) ||
			errno == syscall.Errno(windows.TRUST_E_PROVIDER_UNKNOWN)
	}
	return false
}
