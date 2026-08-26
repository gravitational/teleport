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

package privilegedupdater

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go authenticode_windows.go

//sys CryptMsgGetParam(msg windows.Handle, paramType uint32, index uint32, data *byte, dataLen *uint32) (err error) [failretval==0] = crypt32.CryptMsgGetParam
//sys CryptMsgClose(msg windows.Handle) (err error) [failretval==0] = crypt32.CryptMsgClose

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"slices"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

const (
	// CMSG_SIGNER_CERT_INFO_PARAM retrieves a CERT_INFO structure directly.
	// As in https://learn.microsoft.com/en-us/windows/win32/api/wincrypt/nf-wincrypt-certgetsubjectcertificatefromstore#examples.
	cmsgSignerCertInfoParam = 7
)

// verifySignature checks if the update is signed by Teleport.
func verifySignature(updatePath string) error {
	updatePathPtr, err := windows.UTF16PtrFromString(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = verifyTrust(updatePathPtr); err != nil {
		return trace.Wrap(err, "verifying update signature")
	}
	updateCert, err := getCert(updatePathPtr)
	if err != nil {
		return trace.Wrap(err, "getting update certificate")
	}

	if !hasTeleportSubject(updateCert) {
		return trace.BadParameter("signature verification failed: update subject does not match Teleport subject (teleport: %s, update: %s)", logCert(teleportCert), logCert(updateCert))
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
	closeErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)

	return trace.NewAggregate(err, closeErr)
}

// getCert extracts the certificate context from a signed file.
// This function does not validate the chain of trust. Use verifyTrust for that.
func getCert(path *uint16) (*x509.Certificate, error) {
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
	defer windows.CertFreeCertificateContext(certCtx)

	cert, err := extractX509Certificate(certCtx)
	return cert, trace.Wrap(err)
}

// hasTeleportSubject checks whether updateCert has the expected Teleport publisher subject by comparing a subset of X.509 Subject fields.
// The cert validity must be checked first with verifyTrust.
//
// Security notes:
//   - This does NOT provide cryptographic authenticity guarantees. An attacker could theoretically obtain a certificate
//     with identical subject fields. However, obtaining such a certificate from a CA trusted by Windows is non-trivial
//     (the cert must first pass the WinVerifyTrust check).
//   - Teleport Managed Updates explicitly do not verify asset authenticity (see https://github.com/gravitational/teleport/blob/0bc64ebc163728ece7f9e7b874e6eb9b95736a01/rfd/0184-agent-auto-updates.md?plain=1#L1909-L1918),
//     so this check acts as a best-effort, additional defense layer.
//   - We intentionally avoid pinning a specific certificate in the updater so that certificate renewals or rotations
//     do not accidentally break updates.
//   - This logic should become unnecessary once proper authenticity verification is implemented (e.g., using The Update Framework)
//     along with the supporting infrastructure.
//
// Similar approaches:
//   - electron-updater compares DN attributes:
//     https://github.com/electron-userland/electron-builder/blob/02e59ba8a3b02e1b3ab20035ff43f48ea20880b7/packages/electron-updater/src/windowsExecutableCodeSignatureVerifier.ts#L69-L85
//   - Tailscale verifies only the CN:
//     https://github.com/tailscale/tailscale/blob/3ec5be3f510f74738179c1023468343a62a7e00f/clientupdate/clientupdate_windows.go#L70-L74
func hasTeleportSubject(updateCert *x509.Certificate) bool {
	if updateCert == nil {
		return false
	}

	s1, s2 := teleportCert.Subject, updateCert.Subject
	return s1.CommonName == s2.CommonName &&
		slices.Equal(s1.Organization, s2.Organization) &&
		slices.Equal(s1.Locality, s2.Locality) &&
		slices.Equal(s1.Province, s2.Province) &&
		slices.Equal(s1.Country, s2.Country)
}

var teleportCert = &x509.Certificate{
	Subject: pkix.Name{
		CommonName:   "Gravitational, Inc.",
		Organization: []string{"Gravitational, Inc."},
		Locality:     []string{"Oakland"},
		Province:     []string{"California"},
		Country:      []string{"US"},
	},
}

func logCert(cert *x509.Certificate) string {
	if cert == nil {
		return "<nil>"
	}

	s := cert.Subject
	return fmt.Sprintf("CN=%q, O=%v, L=%v, ST=%v, C=%v", s.CommonName, s.Organization, s.Locality, s.Province, s.Country)
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

func extractX509Certificate(ctx *windows.CertContext) (*x509.Certificate, error) {
	if ctx == nil || ctx.EncodedCert == nil || ctx.Length == 0 {
		return nil, trace.BadParameter("invalid certificate context")
	}

	der := unsafe.Slice(ctx.EncodedCert, int(ctx.Length))
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse x509 certificate")
	}
	return cert, nil
}
