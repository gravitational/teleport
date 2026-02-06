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

package autoupdate

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go authenticode_windows.go

//sys CryptMsgGetParam(msg windows.Handle, paramType uint32, index uint32, data *byte, dataLen *uint32) (err error) [failretval==0] = crypt32.CryptMsgGetParam
//sys CryptMsgClose(msg windows.Handle) (err error) [failretval==0] = crypt32.CryptMsgClose
//sys CertCompareCertificateName(encodingType uint32, name1 *windows.CertNameBlob, name2 *windows.CertNameBlob) (ok bool) = crypt32.CertCompareCertificateName

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

const (
	// CMSG_SIGNER_CERT_INFO_PARAM retrieves a CERT_INFO structure directly.
	// As in https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certgetsubjectcertificatefromstore#examples.
	cmsgSignerCertInfoParam = 7
)

// verifySignature checks if the update is signed by the same entity as the running service.
func verifySignature(updatePath string) error {
	servicePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	servicePathPtr, err := windows.UTF16PtrFromString(servicePath)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = verifyTrust(servicePathPtr); err != nil {
		if errors.Is(err, syscall.Errno(windows.TRUST_E_NOSIGNATURE)) {
			log.Warn("service is not signed, skipping signature verification")
			return nil
		}
		return trace.Wrap(err)
	}

	serviceCert, err := getCertContext(servicePathPtr)
	if err != nil {
		return trace.Wrap(err, "getting service certificate")
	}
	defer windows.CertFreeCertificateContext(serviceCert)

	updatePathPtr, err := windows.UTF16PtrFromString(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = verifyTrust(updatePathPtr); err != nil {
		return trace.Wrap(err, "verifying update signature")
	}
	updateCert, err := getCertContext(updatePathPtr)
	if err != nil {
		return trace.Wrap(err, "getting update certificate")
	}
	defer windows.CertFreeCertificateContext(updateCert)

	if !compareSubjects(serviceCert, updateCert) {
		return trace.BadParameter("signature verification failed: update and service subjects do not match")
	}

	return nil
}

func verifyTrust(path *uint16) error {
	fileInfo := windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: path,
	}
	data := &windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_WHOLECHAIN,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&fileInfo),
	}

	// verify
	err := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)

	// close
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)

	return trace.Wrap(err)
}

// getCertContext extracts the certificate context from a signed file.
// This function does not validate the chain of trust. Use verifyTrust for that.
func getCertContext(path *uint16) (*windows.CertContext, error) {
	var (
		msgHandle, storeHandle windows.Handle
		encoding               uint32
	)

	err := windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(path),
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
		return nil, trace.Wrap(err, "could not open crypt object")
	}
	defer func() {
		windows.CertCloseStore(storeHandle, 0)
		CryptMsgClose(msgHandle)
	}()

	certInfoBlob, err := getCertInfoBlob(msgHandle)
	if err != nil {
		return nil, err
	}

	certInfo := (*windows.CertInfo)(unsafe.Pointer(&certInfoBlob[0]))
	certCtx, err := windows.CertFindCertificateInStore(
		storeHandle,
		encoding,
		0,
		windows.CERT_FIND_SUBJECT_CERT,
		unsafe.Pointer(certInfo),
		nil,
	)
	if err != nil {
		return nil, trace.Wrap(err, "signer certificate not found in store")
	}

	// Caller is responsible for freeing certCtx.
	return certCtx, nil
}

func compareSubjects(ctx1, ctx2 *windows.CertContext) bool {
	const encoding = windows.X509_ASN_ENCODING | windows.PKCS_7_ASN_ENCODING

	return CertCompareCertificateName(encoding, &ctx1.CertInfo.Subject, &ctx2.CertInfo.Subject)
}

func getCertInfoBlob(handle windows.Handle) ([]byte, error) {
	var size uint32
	if err := CryptMsgGetParam(handle, cmsgSignerCertInfoParam, 0, nil, &size); err != nil {
		return nil, trace.Wrap(err, "failed to get CMSG_SIGNER_CERT_INFO_PARAM param size")
	}

	buf := make([]byte, size)
	if err := CryptMsgGetParam(handle, cmsgSignerCertInfoParam, 0, &buf[0], &size); err != nil {
		return nil, trace.Wrap(err, "failed to get CMSG_SIGNER_CERT_INFO_PARAM param data")
	}
	return buf, nil
}
