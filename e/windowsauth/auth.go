package main

/*
#include "auth.h"

void * allocateLsa(PLSA_DISPATCH_TABLE tbl, ULONG size);
void * allocatePrivate(PLSA_DISPATCH_TABLE tbl, ULONG size);
NTSTATUS allocateClient(PLSA_DISPATCH_TABLE tbl, PLSA_CLIENT_REQUEST req, ULONG size, void** out);

extern BOOLEAN AllocateLocallyUniqueId(PLUID Luid);
NTSTATUS copyToClientBuffer(PLSA_DISPATCH_TABLE tbl, PLSA_CLIENT_REQUEST ClientRequest,ULONG Length,void* ClientBaseAddress,void* BufferToCopy);
NTSTATUS createLogonSession(PLSA_DISPATCH_TABLE tbl, PLUID LogonId);
*/
import "C"

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/user"
	"syscall"
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

const (
	// LsaTokenInformationV1 is type of token returned by LsaApLogonUser
	// https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/ne-ntsecpkg-lsa_token_information_type
	LsaTokenInformationV1 = 1

	// RemoteInteractive is security logon type used during interactive logins through RDP
	// https://learn.microsoft.com/en-us/windows/win32/api/ntsecapi/ne-ntsecapi-security_logon_type
	RemoteInteractive = 10

	// SecNameSamCompatible describes secpkg name type in format "Domain\User"
	// https://learn.microsoft.com/pl-pl/windows/win32/api/Ntsecpkg/ne-ntsecpkg-secpkg_name_type
	SecNameSamCompatible = 0

	// SecurityDelegation is the broadest impersonation level
	// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ne-wdm-_security_impersonation_level
	SecurityDelegation = 3
	Success            = 0

	// APName is the name of the Teleport Auth Package.
	// Note: this must be kept in sync with the same value in installer/main.go
	APName = "Teleport"

	statusLogonFailure = uint32(windows.STATUS_LOGON_FAILURE)
)

const (
	invalidLicense = iota + 1
	noSmartCardKeyUsage
	invalidCertificate
	unknownError
)

// maxComputerName length is the maximum number of bytes (not characters)
// for a computer name. See https://learn.microsoft.com/en-us/troubleshoot/windows-server/identity/naming-conventions-for-computer-domain-site-ou
const maxComputerNameLength = 32

var smartCardLogonKeyUsage = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 2}

// LSATokenInformation is Go version of LSA_TOKEN_INFORMATION_V1 structure
// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/legacy/aa378721(v=vs.85)
type LSATokenInformation struct {
	ExpirationTime uint64
	User           syscall.Tokenuser
	Groups         *windows.Tokengroups
	PrimaryGroup   syscall.Tokenprimarygroup
	Privileges     unsafe.Pointer
	Owner          syscall.Tokenuser
	DefaultDacl    unsafe.Pointer
}

//export LsaApLogonUser
func LsaApLogonUser(clientRequest C.PLSA_CLIENT_REQUEST, logonType uint32, authenticationInformation,
	clientAuthenticationBase *byte, authenticationInformationLength uint32, profileBuffer *unsafe.Pointer,
	profileBufferLength *C.ulong, logonId C.PLUID, subStatus *uint32, tokenInformationType *uint32,
	tokenInformation *unsafe.Pointer, accountName, authenticatingAuthority *C.PUNICODE_STRING) uint32 {
	return lsaApLogonUser(clientRequest, logonType, authenticationInformation, clientAuthenticationBase,
		authenticationInformationLength, profileBuffer, profileBufferLength, logonId, subStatus, tokenInformationType,
		tokenInformation, accountName, authenticatingAuthority)
}

// lsaApLogonUser authenticates a user's logon credentials.
// https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_logon_user
func lsaApLogonUser(clientRequest C.PLSA_CLIENT_REQUEST, logonType uint32, authenticationInformation, _ *byte,
	authenticationInformationLength uint32, profileBuffer *unsafe.Pointer, profileBufferLength *C.ulong,
	logonId C.PLUID, subStatus *uint32, tokenInformationType *uint32, tokenInformation *unsafe.Pointer,
	accountName, authenticatingAuthority *C.PUNICODE_STRING) uint32 {
	log.WithField("logon type", logonType).Info("logging user")
	*subStatus = invalidCertificate

	if logonType != RemoteInteractive {
		log.Errorf("invalid login type %v: only remote interactive logon supported", logonType)
		return statusLogonFailure
	}

	if authenticationInformationLength < 16 {
		log.WithField("length", authenticationInformationLength).Error("authentication information too short")
		return statusLogonFailure
	}
	bytes := unsafe.Slice(authenticationInformation, authenticationInformationLength)
	cert, err := x509.ParseCertificate(bytes[16:])
	if err != nil {
		log.WithError(err).Error("can't parse certificate")
		return statusLogonFailure
	}

	if _, err := cert.Verify(x509.VerifyOptions{KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}}); err != nil {
		log.WithError(err).Error("can't verify certificate")
		return statusLogonFailure
	}

	if !hasSmartCardKeyUsage(cert) {
		log.Error("no smart card key usage")
		*subStatus = noSmartCardKeyUsage
		return statusLogonFailure
	}

	if !hasEnterpriseLicense(cert) {
		*subStatus = invalidLicense
		log.Error("no enterprise license")
		return statusLogonFailure
	}

	raph := windows.Handle(binary.LittleEndian.Uint64(bytes))
	waph := windows.Handle(binary.LittleEndian.Uint64(bytes[8:]))
	if err := verifyChallenge(raph, waph, cert); err != nil {
		log.WithError(err).Error("can't verify challenge")
		return statusLogonFailure
	}

	*subStatus = unknownError

	name := cert.Subject.CommonName

	// We don't populate the profile buffer, but Windows requires that we
	// at least allocate some memory. (This memory is freed by LSA)
	*profileBufferLength = C.ulong(256)
	if status := C.allocateClient(DispatchTable, clientRequest, *profileBufferLength, profileBuffer); status != Success {
		log.WithField("status", status).Error("can't allocate profile buffer")
		return statusLogonFailure
	}
	var zero [256]byte
	if status := C.copyToClientBuffer(DispatchTable, clientRequest, *profileBufferLength, *profileBuffer,
		unsafe.Pointer(&zero[0])); status != Success {
		log.WithField("status", status).Error("can't zero profile buffer")
		return statusLogonFailure
	}

	account, err := user.Lookup(name)
	if err != nil {
		log.WithFields(log.Fields{log.ErrorKey: err, "name": name}).Error("can't lookup user")
		return uint32(windows.STATUS_NO_SUCH_USER)
	}
	sid, err := syscall.StringToSid(account.Uid)
	if err != nil {
		log.WithFields(log.Fields{log.ErrorKey: err, "sid": account.Uid}).Error("can't convert user SID")
		return statusLogonFailure
	}
	primaryGroupSid, err := syscall.StringToSid(account.Gid)
	if err != nil {
		log.WithFields(log.Fields{log.ErrorKey: err, "sid": account.Uid}).Error("can't convert primary group SID")
		return statusLogonFailure
	}

	groupIds, err := account.GroupIds()
	if err != nil {
		log.WithError(err).Error("can't get user groups")
		return statusLogonFailure
	}

	log.WithFields(log.Fields{
		"user":     account.Uid,
		"primary":  account.Gid,
		"groups":   groupIds,
		"username": account.Username,
	}).Info("account")

	if status, err := C.AllocateLocallyUniqueId(logonId); status == 0 {
		log.WithError(err).Error("can't allocate logon id")
		return statusLogonFailure
	}
	if status := C.createLogonSession(DispatchTable, logonId); status != Success {
		log.WithError(err).Error("can't create logon session")
		return statusLogonFailure
	}

	*tokenInformationType = LsaTokenInformationV1
	*tokenInformation = C.allocateLsa(DispatchTable, C.ulong(unsafe.Sizeof(LSATokenInformation{})))
	tokenInfo := (*LSATokenInformation)(*tokenInformation)
	tokenInfo.ExpirationTime = math.MaxUint64

	tokenInfo.User.User.Sid = (*syscall.SID)(C.allocateLsa(DispatchTable, (C.ulong)(sid.Len())))
	if err := syscall.CopySid(uint32(sid.Len()), tokenInfo.User.User.Sid, sid); err != nil {
		log.WithError(err).Error("can't copy user SID")
		return statusLogonFailure
	}

	tokenInfo.PrimaryGroup.PrimaryGroup = (*syscall.SID)(C.allocateLsa(DispatchTable, (C.ulong)(primaryGroupSid.Len())))
	if err := syscall.CopySid(uint32(primaryGroupSid.Len()), tokenInfo.PrimaryGroup.PrimaryGroup, primaryGroupSid); err != nil {
		log.WithError(err).Error("can't copy primary group SID")
		return statusLogonFailure
	}

	groupsSize := 4 + uint32(len(groupIds)*int(unsafe.Sizeof(windows.SIDAndAttributes{})))
	tokenInfo.Groups = (*windows.Tokengroups)(C.allocatePrivate(DispatchTable, C.ulong(groupsSize)))
	tokenInfo.Groups.GroupCount = uint32(len(groupIds))
	targetGroups := tokenInfo.Groups.AllGroups()

	for i, group := range groupIds {
		gsid, err := syscall.StringToSid(group)
		if err != nil {
			log.WithFields(log.Fields{log.ErrorKey: err, "sid": group}).Error("can't convert group SID")
			return statusLogonFailure
		}
		targetGroups[i].Attributes = windows.SE_GROUP_MANDATORY | windows.SE_GROUP_ENABLED | windows.SE_GROUP_ENABLED_BY_DEFAULT
		targetGroups[i].Sid = (*windows.SID)(C.allocatePrivate(DispatchTable, C.ulong(gsid.Len())))
		if err := syscall.CopySid(uint32(gsid.Len()), (*syscall.SID)(targetGroups[i].Sid), gsid); err != nil {
			log.WithError(err).WithField("SID", gsid).Error("can't copy group")
			return statusLogonFailure
		}
	}

	*accountName, err = toLSAString(name)
	if err != nil {
		log.WithError(err).Error("can't create accountName")
		return statusLogonFailure
	}

	var domain [maxComputerNameLength]uint16
	l := uint32(maxComputerNameLength)
	if err := windows.GetComputerName(&domain[0], &l); err != nil {
		log.WithError(err).Error("can't get computer name")
		return statusLogonFailure
	}
	*authenticatingAuthority, err = toLSAString(windows.UTF16PtrToString(&domain[0]))
	if err != nil {
		log.WithError(err).Error("can't create authenticatingAuthority")
		return statusLogonFailure
	}

	*subStatus = uint32(windows.STATUS_SUCCESS)
	return *subStatus
}

var licenseOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 14}

func hasEnterpriseLicense(cert *x509.Certificate) bool {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(licenseOID) {
			return string(ext.Value) == "ent"
		}
	}
	return false
}

func hasSmartCardKeyUsage(cert *x509.Certificate) bool {
	for _, usage := range cert.UnknownExtKeyUsage {
		if usage.Equal(smartCardLogonKeyUsage) {
			return true
		}
	}
	return false
}

func verifyChallenge(rh, wh windows.Handle, cert *x509.Certificate) error {
	defer windows.CloseHandle(rh)
	defer windows.CloseHandle(wh)
	rhf := os.NewFile(uintptr(rh), "rh")
	if rhf == nil {
		return errors.New("can't open read pipe descriptor")
	}
	whf := os.NewFile(uintptr(wh), "wh")
	if whf == nil {
		return fmt.Errorf("can't open write pipe descriptor")
	}

	data := make([]byte, 256)
	if _, err := rand.Read(data); err != nil {
		return fmt.Errorf("can't generate data: %w", err)
	}

	if _, err := whf.Write(data); err != nil {
		return fmt.Errorf("can't write data: %w", err)
	}
	if err := windows.CloseHandle(wh); err != nil {
		return err
	}

	sign := make([]byte, 256)
	if _, err := io.ReadFull(rhf, sign); err != nil {
		return fmt.Errorf("can't read signature: %w", err)
	}

	sum := sha1.Sum(data)
	key := cert.PublicKey.(*rsa.PublicKey)
	return rsa.VerifyPKCS1v15(key, crypto.SHA1, sum[:], sign)
}

var DispatchTable C.PLSA_DISPATCH_TABLE

//export LsaApInitializePackage
func LsaApInitializePackage(authenticationPackageId C.USHORT, lsaDispatchTable C.PLSA_DISPATCH_TABLE, database C.PLSA_STRING,
	confidentiality C.PLSA_STRING, authenticationPackageName **C.LSA_STRING) C.NTSTATUS {
	return lsaApInitializePackage(authenticationPackageId, lsaDispatchTable, database, confidentiality, authenticationPackageName)
}

// lsaApInitializePackage is called once by LSA during system startup to provide package chance to initialize itself.
// It also provides table with useful functions
// https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_initialize_package
func lsaApInitializePackage(authenticationPackageId C.USHORT, lsaDispatchTable C.PLSA_DISPATCH_TABLE, _, _ C.PLSA_STRING,
	authenticationPackageName **C.LSA_STRING) C.NTSTATUS {
	log.WithField("authPackageId", authenticationPackageId).Info("initializing")
	DispatchTable = lsaDispatchTable
	*authenticationPackageName = (*C.LSA_STRING)(C.allocateLsa(lsaDispatchTable, C.sizeof_LSA_STRING))
	(**authenticationPackageName).Length = C.ushort(len(APName))
	(**authenticationPackageName).MaximumLength = C.ushort(len(APName))
	(**authenticationPackageName).Buffer = C.CString(APName)
	return 0
}

// LsaApCallPackage* functions are used in non-interactive scenarios, we don't support them here, but they have to be present in every AP

//export LsaApCallPackage
func LsaApCallPackage(clientRequest C.PLSA_CLIENT_REQUEST, protocolSubmitBuffer, clientBufferBase unsafe.Pointer,
	submitBufferLength C.ULONG, protocolReturnBuffer *unsafe.Pointer, returnBufferLength *C.ULONG, protocolStatus *C.NTSTATUS) uint32 {
	return uint32(windows.STATUS_UNSUCCESSFUL)
}

//export LsaApCallPackagePassthrough
func LsaApCallPackagePassthrough(clientRequest *unsafe.Pointer, protocolSubmitBuffer, clientBufferBase unsafe.Pointer,
	submitBufferLength C.ULONG, protocolReturnBuffer *unsafe.Pointer, returnBufferLength *C.ULONG, protocolStatus *C.NTSTATUS) uint32 {
	return uint32(windows.STATUS_UNSUCCESSFUL)
}

//export LsaApCallPackageUntrusted
func LsaApCallPackageUntrusted(clientRequest *unsafe.Pointer, protocolSubmitBuffer, clientBufferBase unsafe.Pointer,
	submitBufferLength C.ULONG, protocolReturnBuffer *unsafe.Pointer, returnBufferLength *C.ULONG, protocolStatus *C.NTSTATUS) uint32 {
	return uint32(windows.STATUS_UNSUCCESSFUL)
}

//export LsaApLogonTerminated
func LsaApLogonTerminated(logonId unsafe.Pointer) {
	lsaApLogonTerminated(logonId)
}

// lsaApLogonTerminated is called when logon session started by this package ends i.e. user logs out.
// https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_logon_terminated
func lsaApLogonTerminated(logonId unsafe.Pointer) {
}

func toLSAString(s string) (C.PUNICODE_STRING, error) {
	enc, err := windows.NewNTUnicodeString(s)
	if err != nil {
		return nil, err
	}
	str := (C.PUNICODE_STRING)(C.allocateLsa(DispatchTable, C.sizeof_UNICODE_STRING))
	str.MaximumLength = C.ushort(enc.MaximumLength)
	str.Length = C.ushort(enc.Length)
	str.Buffer = (C.PWSTR)(C.allocateLsa(DispatchTable, C.ulong(str.MaximumLength)))
	// MaximumLength is in bytes, so we'll treat both buffers as byte slices to avoid subtle bugs
	src := unsafe.Slice((*byte)(unsafe.Pointer(enc.Buffer)), int(str.MaximumLength))
	dst := unsafe.Slice((*byte)(unsafe.Pointer(str.Buffer)), int(str.MaximumLength))
	copy(dst, src)
	return str, nil
}
