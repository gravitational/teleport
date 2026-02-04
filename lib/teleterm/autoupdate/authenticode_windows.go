package autoupdate

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/windows"
)

func verifySignature(updatePath string) error {
	servicePath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	serviceSigned, serviceSubject, err := signerSubject(servicePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if !serviceSigned || serviceSubject == "" {
		log.Info("Service binary not signed; skipping installer signature verification")
		return nil
	}

	updateSigned, updateSubject, err := signerSubject(updatePath)
	if err != nil {
		return trace.Wrap(err)
	}
	if !updateSigned {
		return trace.BadParameter("installer signature is not valid")
	}
	if updateSubject == "" {
		return trace.BadParameter("installer signature subject is empty")
	}
	if updateSubject != serviceSubject {
		return trace.BadParameter("installer signature subject does not match service signature")
	}
	return nil
}

func signerSubject(path string) (bool, string, error) {
	signed, err := verifyAuthenticode(path)
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	if !signed {
		return false, "", nil
	}

	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, "", trace.Wrap(err)
	}

	var encoding, contentType, format uint32
	var msgHandle windows.Handle
	var certStore windows.Handle

	if err := windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(path16),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0,
		&encoding,
		&contentType,
		&format,
		&certStore,
		&msgHandle,
		nil,
	); err != nil {
		return false, "", trace.Wrap(err)
	}
	defer windows.CertCloseStore(certStore, 0)
	defer cryptMsgClose(msgHandle)

	signerInfo, err := signerInfoFromMessage(msgHandle)
	if err != nil {
		return false, "", trace.Wrap(err)
	}

	certInfo := certInfo{
		Issuer:       signerInfo.Issuer,
		SerialNumber: signerInfo.SerialNumber,
	}
	certCtx, err := windows.CertFindCertificateInStore(
		certStore,
		encoding,
		0,
		windows.CERT_FIND_SUBJECT_CERT,
		unsafe.Pointer(&certInfo),
		nil,
	)
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	defer windows.CertFreeCertificateContext(certCtx)

	subject, err := certSubject(certCtx)
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	return true, subject, nil
}

func verifyAuthenticode(path string) (bool, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, trace.Wrap(err)
	}
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
	err = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	if err == nil {
		return true, nil
	}
	if isNoSignature(err) {
		return false, nil
	}
	return false, err
}

func isNoSignature(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errors.Is(errno, syscall.Errno(windows.TRUST_E_NOSIGNATURE)) ||
		errors.Is(errno, syscall.Errno(windows.TRUST_E_SUBJECT_FORM_UNKNOWN))
}

func signerInfoFromMessage(msgHandle windows.Handle) (*cmsgSignerInfo, error) {
	var size uint32
	if err := cryptMsgGetParam(msgHandle, cmsgSignerInfoParam, 0, nil, &size); err != nil {
		return nil, trace.Wrap(err)
	}
	buf := make([]byte, size)
	if err := cryptMsgGetParam(msgHandle, cmsgSignerInfoParam, 0, &buf[0], &size); err != nil {
		return nil, trace.Wrap(err)
	}
	return (*cmsgSignerInfo)(unsafe.Pointer(&buf[0])), nil
}

func certSubject(certCtx *windows.CertContext) (string, error) {
	nameLen := windows.CertGetNameString(certCtx, windows.CERT_NAME_RDN_TYPE, 0, nil, nil, 0)
	if nameLen <= 1 {
		return "", nil
	}
	name := make([]uint16, nameLen)
	windows.CertGetNameString(certCtx, windows.CERT_NAME_RDN_TYPE, 0, nil, &name[0], nameLen)
	return windows.UTF16ToString(name), nil
}

const cmsgSignerInfoParam = 6

var crypt32 = windows.NewLazySystemDLL("crypt32.dll")
var procCryptMsgGetParam = crypt32.NewProc("CryptMsgGetParam")
var procCryptMsgClose = crypt32.NewProc("CryptMsgClose")

func cryptMsgGetParam(msgHandle windows.Handle, paramType uint32, index uint32, data *byte, dataLen *uint32) error {
	r1, _, e1 := syscall.SyscallN(
		procCryptMsgGetParam.Addr(),
		uintptr(msgHandle),
		uintptr(paramType),
		uintptr(index),
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(dataLen)),
	)
	if r1 != 0 {
		return nil
	}
	if e1 != 0 {
		return error(e1)
	}
	return syscall.EINVAL
}

func cryptMsgClose(msgHandle windows.Handle) {
	_, _, _ = syscall.SyscallN(procCryptMsgClose.Addr(), uintptr(msgHandle))
}

type cryptDataBlob struct {
	cbData uint32
	pbData *byte
}

type cryptObjIDBlob = cryptDataBlob
type certNameBlob = cryptDataBlob
type cryptIntegerBlob = cryptDataBlob

type cryptAlgorithmIdentifier struct {
	pszObjId *byte
	Params   cryptObjIDBlob
}

type cryptAttribute struct {
	pszObjId *byte
	cValue   uint32
	rgValue  *cryptDataBlob
}

type cryptAttributes struct {
	cAttr  uint32
	rgAttr *cryptAttribute
}

type cmsgSignerInfo struct {
	dwVersion         uint32
	Issuer            certNameBlob
	SerialNumber      cryptIntegerBlob
	HashAlgorithm     cryptAlgorithmIdentifier
	HashEncryptionAlg cryptAlgorithmIdentifier
	EncryptedHash     cryptDataBlob
	AuthAttrs         cryptAttributes
	UnauthAttrs       cryptAttributes
}

type cryptBitBlob struct {
	cbData      uint32
	pbData      *byte
	cUnusedBits uint32
}

type certPublicKeyInfo struct {
	Algorithm cryptAlgorithmIdentifier
	PublicKey cryptBitBlob
}

type certExtension struct {
	pszObjId  *byte
	fCritical int32
	Value     cryptObjIDBlob
}

type certInfo struct {
	dwVersion            uint32
	SerialNumber         cryptIntegerBlob
	SignatureAlgorithm   cryptAlgorithmIdentifier
	Issuer               certNameBlob
	NotBefore            windows.Filetime
	NotAfter             windows.Filetime
	Subject              certNameBlob
	SubjectPublicKeyInfo certPublicKeyInfo
	IssuerUniqueId       cryptBitBlob
	SubjectUniqueId      cryptBitBlob
	cExtension           uint32
	rgExtension          *certExtension
}
