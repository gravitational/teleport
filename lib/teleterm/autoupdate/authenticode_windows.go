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

var (
	procCertCompareCertificateName = crypt32.NewProc("CertCompareCertificateName")
)

// VerifySignature checks if the update is signed by the same entity as the running service.
func VerifySignature(updatePath string) error {
	servicePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// 2. Get Certificate Context for the Update
	updateCert, err := getCertContext(updatePath)
	if err != nil {
		return fmt.Errorf("update verification failed: %w", err)
	}
	defer windows.CertFreeCertificateContext(updateCert)

	// 1. Get Certificate Context for the running Service
	serviceCert, err := getCertContext(servicePath)
	if err != nil {
		// If the service isn't signed, we decide here if that's an error.
		// For dev builds, you might want to return nil.
		return fmt.Errorf("current service verification failed: %w", err)
	}
	defer windows.CertFreeCertificateContext(serviceCert)

	// 3. Compare the Subjects using CertCompareCertificateName
	// This compares the ASN.1 binary blobs directly.
	if !compareSubjects(serviceCert, updateCert) {
		return fmt.Errorf("signature mismatch: update is not signed by the same entity as the service")
	}

	return nil
}

func compareSubjects(ctx1, ctx2 *windows.CertContext) bool {
	// X509_ASN_ENCODING | PKCS_7_ASN_ENCODING
	const encoding = 0x00010001

	r1, _, err := syscall.Syscall6(
		procCertCompareCertificateName.Addr(),
		3,
		uintptr(encoding),
		uintptr(unsafe.Pointer(&ctx1.CertInfo.Subject)),
		uintptr(unsafe.Pointer(&ctx2.CertInfo.Subject)),
		0, 0, 0,
	)

	if err != 0 {
		log.Info("failed to compare subjects, %s", err)
	}

	return r1 != 0 // Returns 1 (TRUE) if identical
}

// getCertContext extracts the leaf certificate context from a signed file.
// It verifies the Authenticode signature first.
func getCertContext(path string) (*windows.CertContext, error) {
	// 1. Integrity Check
	if err := verifyAuthenticode(path); err != nil {
		return nil, err
	}

	path16, _ := windows.UTF16PtrFromString(path)
	var (
		msgHandle, storeHandle windows.Handle
		encoding               uint32
	)

	// 2. Open Crypt Object
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
		return nil, fmt.Errorf("CryptQueryObject: %w", err)
	}
	defer windows.CertCloseStore(storeHandle, 0)
	defer cryptMsgClose(msgHandle)

	// 3. Extract Signer INFO Blob (Using Param 7 for efficiency)
	certInfoBlob, err := getCryptMsgParam(msgHandle, 7) // CMSG_SIGNER_CERT_INFO_PARAM
	if err != nil {
		return nil, err
	}

	// 4. Find the Certificate in the Store
	certCtx, err := windows.CertFindCertificateInStore(
		storeHandle,
		encoding,
		0,
		windows.CERT_FIND_SUBJECT_CERT,
		unsafe.Pointer(&certInfoBlob[0]),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("signer certificate not found in store: %w", err)
	}

	// Caller is responsible for freeing certCtx
	return certCtx, nil
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
