package main

/*
#include "auth.h"

void * allocateLsa(PLSA_DISPATCH_TABLE tbl, ULONG size);
void * allocatePrivate(PLSA_DISPATCH_TABLE tbl, ULONG size);
NTSTATUS allocateClient(PLSA_DISPATCH_TABLE tbl, PLSA_CLIENT_REQUEST req, ULONG size, void** out);

NTSTATUS copyToClientBuffer(PLSA_DISPATCH_TABLE tbl, PLSA_CLIENT_REQUEST ClientRequest,ULONG Length,void* ClientBaseAddress,void* BufferToCopy);
NTSTATUS createLogonSession(PLSA_DISPATCH_TABLE tbl, void* LogonId);
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
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"io"
	"math"
	"os"
	"os/user"
	"sync"
	"syscall"
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// see mksyscall.go
//sys NetUserAdd(servername *uint16, level uint32, buf *userInfo, errIndex *uint32) (err error) [failretval!=0] = Netapi32.NetUserAdd
//sys NetUserDel(servername *uint16, user *uint16) (err error) [failretval!=0] = Netapi32.NetUserDel
//sys NetLocalGroupAddMembers(servername *uint16, group *uint16, level uint32, members *membersInfo, totalEntries uint32) (err error) [failretval!=0] = Netapi32.NetLocalGroupAddMembers
//sys NetLocalGroupAdd(servername *uint16, level uint32, buf *membersInfo, errIndex *uint32) (err error) [failretval!=0] = Netapi32.NetLocalGroupAdd
//sys NetLocalGroupDel(servername *uint16, group *uint16) (err error) [failretval!=0] = Netapi32.NetLocalGroupDel
//sys NetUserSetFlags(servername *uint16, username *uint16, level uint32, buf *flags, errIndex *uint32) (err error) [failretval!=0] = Netapi32.NetUserSetInfo
//sys AllocateLocallyUniqueId(pluid *windows.LUID) (err error) = Advapi32.AllocateLocallyUniqueId

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

// Constants used in NetUserSetInfo calls. See https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1008
const (
	flagsLevel      = 1008
	scriptExecuted  = 1
	accountDisabled = 2
)

// maxComputerName length is the maximum number of bytes (not characters)
// for a computer name. See https://learn.microsoft.com/en-us/troubleshoot/windows-server/identity/naming-conventions-for-computer-domain-site-ou
const maxComputerNameLength = 32

const (
	// teleportUsers is the name of the group used for bookkeeping, all users created by Teleport will be part of this group
	teleportUsers = "Teleport Users"

	// remoteDesktopUsers is group required for RDP connections to work, we will add it automatically to all users we manage
	remoteDesktopUsers = "Remote Desktop Users"
)

var smartCardLogonKeyUsage = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 20, 2, 2}

var createUserOID = asn1.ObjectIdentifier{1, 3, 9999, 2, 16}

// LSATokenInformation is Go version of LSA_TOKEN_INFORMATION_V1 structure
// https://learn.microsoft.com/en-us/previous-versions/windows/desktop/legacy/aa378721(v=vs.85)
type LSATokenInformation struct {
	expirationTime uint64
	user           syscall.Tokenuser
	groups         *windows.Tokengroups
	primaryGroup   syscall.Tokenprimarygroup
	privileges     unsafe.Pointer
	owner          syscall.Tokenuser
	defaultDacl    unsafe.Pointer
}

// userInfo is Go version of USER_INFO_1 structure
// https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1
type userInfo struct {
	name        *uint16
	password    *uint16
	passwordAge uint32
	priv        uint32
	homeDir     *uint16
	comment     *uint16
	flags       uint32
	scriptPath  *uint16
}

// flags is Go version of USER_INFO_1008 structure
// https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-user_info_1008
type flags struct {
	flags uint32
}

// membersInfo is Go version of two structures: LOCALGROUP_MEMBERS_INFO_3 and LOCALGROUP_INFO_0
// https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-localgroup_members_info_3
// https://learn.microsoft.com/en-us/windows/win32/api/lmaccess/ns-lmaccess-localgroup_info_0
type membersInfo struct {
	domainAndName *uint16
}

// shouldCreateUser checks if certificate contains createUserOID and will extract from it if we should create user and
// which groups the user should be part of.
func shouldCreateUser(cert *x509.Certificate) (bool, []string) {
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(createUserOID) {
			log.Info("Found create user OID")
			var data struct {
				CreateUser bool     `json:"createUser"`
				Groups     []string `json:"groups"`
			}
			if err := json.Unmarshal(ext.Value, &data); err != nil {
				log.WithError(err).Error("can't unmarshall")
				return false, nil
			}
			return data.CreateUser, append(data.Groups, remoteDesktopUsers, teleportUsers)
		}
	}
	log.Info("No create user OID")
	return false, nil
}

func createGroups(groups []string) error {
	for _, group := range groups {
		if _, err := user.LookupGroup(group); err != nil {
			log.WithField("group", group).Info("creating group")
			uname, err := windows.UTF16PtrFromString(group)
			if err != nil {
				return fmt.Errorf("can't convert group name: %w", err)
			}
			if err := NetLocalGroupAdd(nil, 0, &membersInfo{domainAndName: uname}, nil); err != nil {
				return fmt.Errorf("can't add group %q: %w", group, err)
			}
		}
	}
	return nil
}

// ensureUser will create user if it's missing and will add this new user to teleportUsers group to mark it as managed
// by Teleport.
func ensureUser(name string) error {
	if _, err := user.Lookup(name); err == nil {
		return nil
	}

	uname, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("can't convert name: %w", err)
	}

	if err = NetUserAdd(nil, 1, &userInfo{name: uname, priv: 1}, nil); err != nil {
		err = fmt.Errorf("can't create user: %w", err)
		return err
	}

	account, err := user.Lookup(name)
	if err != nil {
		return fmt.Errorf("can't lookup user: %w", err)
	}

	if _, err := user.LookupGroup(teleportUsers); err != nil {
		if err := createGroups([]string{teleportUsers}); err != nil {
			return fmt.Errorf("can't create Teleport Users group: %w", err)
		}
	}

	if err := addToGroup(account, teleportUsers); err != nil {
		return err
	}

	return nil
}

func addToGroup(account *user.User, group string) error {
	log.WithFields(log.Fields{
		"name":  account.Name,
		"group": group,
	}).Info("adding to group")
	domainAndName, err := windows.UTF16PtrFromString(account.Username)
	if err != nil {
		return err
	}

	ugroup, err := windows.UTF16PtrFromString(group)
	if err != nil {
		return err
	}

	if err := NetLocalGroupAddMembers(nil, ugroup, 3, &membersInfo{domainAndName}, 1); err != nil {
		return fmt.Errorf("can't add user to group: %w", err)
	}
	return nil
}

//export LsaApLogonUser
func LsaApLogonUser(clientRequest C.PLSA_CLIENT_REQUEST, logonType uint32, authenticationInformation,
	clientAuthenticationBase *byte, authenticationInformationLength uint32, profileBuffer *unsafe.Pointer,
	profileBufferLength *C.ulong, logonId unsafe.Pointer, subStatus *uint32, tokenInformationType *uint32,
	tokenInformation *unsafe.Pointer, accountName, authenticatingAuthority *C.PUNICODE_STRING) uint32 {
	return lsaApLogonUser(clientRequest, logonType, authenticationInformation, clientAuthenticationBase,
		authenticationInformationLength, profileBuffer, profileBufferLength, (*windows.LUID)(logonId),
		subStatus, tokenInformationType, tokenInformation, accountName, authenticatingAuthority)
}

// sessions represent logon sessions managed by this Authentication Package
type sessions struct {
	mu            sync.Mutex
	luidsToNames  map[windows.LUID]string
	namesToCounts map[string]int
}

// start associates session logon ID and username and increases number of active sessions for that username.
// Returns true if starting fresh session i.e. count of active sessions for the username was 0 before this call.
func (s *sessions) start(luid windows.LUID, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.luidsToNames[luid] = name
	s.namesToCounts[name] += 1
	return s.namesToCounts[name] == 1
}

// end marks end of the session for specified logon ID, decreasing number of active sessions for associated username.
// Returns true if there was username matching the logon ID, and it was last session for this user i.e. active sessions
// count is 0 after this call
func (s *sessions) end(luid windows.LUID) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name, ok := s.luidsToNames[luid]
	if !ok {
		return "", false
	}
	s.namesToCounts[name] -= 1
	return name, s.namesToCounts[name] == 0
}

var logonSessions = sessions{
	luidsToNames:  make(map[windows.LUID]string),
	namesToCounts: make(map[string]int),
}

// lsaApLogonUser authenticates a user's logon credentials.
// https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_logon_user
func lsaApLogonUser(clientRequest C.PLSA_CLIENT_REQUEST, logonType uint32, authenticationInformation, _ *byte,
	authenticationInformationLength uint32, profileBuffer *unsafe.Pointer, profileBufferLength *C.ulong,
	logonId *windows.LUID, subStatus *uint32, tokenInformationType *uint32, tokenInformation *unsafe.Pointer,
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

	if err := AllocateLocallyUniqueId(logonId); err != nil {
		log.WithError(err).Error("can't allocate logon id")
		return statusLogonFailure
	}

	var groupIds []string

	createUser, groups := shouldCreateUser(cert)
	if createUser {
		if err := ensureUser(name); err != nil {
			log.WithError(err).Error("can't create user")
			return statusLogonFailure
		}
		teleportGroup, err := user.LookupGroup(teleportUsers)
		if err != nil {
			log.WithError(err).Error("can't find Teleport Users group")
			return statusLogonFailure
		}
		account, err := user.Lookup(name)
		if err != nil {
			log.WithError(err).Error("can't lookup user")
			return statusLogonFailure
		}
		groupIds, err = account.GroupIds()
		if err != nil {
			log.WithError(err).Error("can't get user groups")
			return statusLogonFailure
		}

		if slices.Contains(groupIds, teleportGroup.Gid) {
			// user is part of Teleport Users group i.e. managed by Teleport

			// create all requested groups
			if err := createGroups(groups); err != nil {
				log.WithError(err).Error("can't create groups")
				return statusLogonFailure
			}

			// gather SIDs for all requested groups
			groupIds = nil
			for _, g := range groups {
				group, err := user.LookupGroup(g)
				if err != nil {
					log.WithError(err).Errorf("can't lookup group %s", group)
					return statusLogonFailure
				}
				groupIds = append(groupIds, group.Gid)
			}
		}
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

	// if the user is not managed by Teleport we gather groups from the system
	if groupIds == nil {
		groupIds, err = account.GroupIds()
		if err != nil {
			log.WithError(err).Error("can't get user groups")
			return statusLogonFailure
		}
	}

	log.WithFields(log.Fields{
		"user":     account.Uid,
		"primary":  account.Gid,
		"groups":   groupIds,
		"username": account.Username,
	}).Info("account")

	if status := C.createLogonSession(DispatchTable, unsafe.Pointer(logonId)); status != Success {
		log.WithError(err).Error("can't create logon session")
		return statusLogonFailure
	}

	log.WithFields(log.Fields{
		"name": name,
		"luid": logonId,
	}).Info("LUID")

	*tokenInformationType = LsaTokenInformationV1
	*tokenInformation = C.allocateLsa(DispatchTable, C.ulong(unsafe.Sizeof(LSATokenInformation{})))
	tokenInfo := (*LSATokenInformation)(*tokenInformation)
	tokenInfo.expirationTime = math.MaxUint64

	tokenInfo.user.User.Sid = (*syscall.SID)(C.allocateLsa(DispatchTable, (C.ulong)(sid.Len())))
	if err := syscall.CopySid(uint32(sid.Len()), tokenInfo.user.User.Sid, sid); err != nil {
		log.WithError(err).Error("can't copy user SID")
		return statusLogonFailure
	}

	tokenInfo.primaryGroup.PrimaryGroup = (*syscall.SID)(C.allocateLsa(DispatchTable, (C.ulong)(primaryGroupSid.Len())))
	if err := syscall.CopySid(uint32(primaryGroupSid.Len()), tokenInfo.primaryGroup.PrimaryGroup, primaryGroupSid); err != nil {
		log.WithError(err).Error("can't copy primary group SID")
		return statusLogonFailure
	}

	groupsSize := 4 + uint32(len(groupIds)*int(unsafe.Sizeof(windows.SIDAndAttributes{})))
	tokenInfo.groups = (*windows.Tokengroups)(C.allocatePrivate(DispatchTable, C.ulong(groupsSize)))
	tokenInfo.groups.GroupCount = uint32(len(groupIds))
	targetGroups := tokenInfo.groups.AllGroups()

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
	lsaApLogonTerminated((*windows.LUID)(logonId))
}

// lsaApLogonTerminated is called when logon session started by this package ends i.e. user logs out.
// https://learn.microsoft.com/en-us/windows/win32/api/ntsecpkg/nc-ntsecpkg-lsa_ap_logon_terminated
func lsaApLogonTerminated(logonId *windows.LUID) {
	log.WithFields(log.Fields{"luid": *logonId}).Info("terminated")
	if name, last := logonSessions.end(*logonId); last {
		//this was last session for the user, disable it
		log.WithField("name", name).Info("disabling user")
		if err := setUserStatus(name, false); err != nil {
			log.WithError(err).Error("can't disable user")
		}
	}
}

// setUserStatus can make user disabled (they can't log in) or enabled.
func setUserStatus(name string, active bool) error {
	uname, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("can't convert name: %w", err)
	}
	newFlags := uint32(scriptExecuted)
	if !active {
		newFlags = newFlags | accountDisabled
	}
	return NetUserSetFlags(nil, uname, flagsLevel, &flags{flags: newFlags}, nil)
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
