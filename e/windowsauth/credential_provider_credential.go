package main

/*
#include "com.h"

unsigned int credentialAdvise(void* this, void *pcpce);
unsigned int unAdvise(void* this);
unsigned int setSelected(void* this, int* pbAutoLogon);
unsigned int setDeselected(void* this);
unsigned int getFieldState(void* this, unsigned int dwFieldID, unsigned int* pcpfs, unsigned int* pcpfis);
unsigned int getStringValue(void* this, unsigned int dwFieldID, void* ppsz);
unsigned int getBitmapValue(void* this, unsigned int dwFieldID, void* *phbmp);
unsigned int getCheckboxValue(void* this, unsigned int dwFieldID, int* pbChecked, void* ppszLabel);
unsigned int getSubmitButtonValue(void* this, unsigned int dwFieldID, unsigned int* pdwAdjacentTo);
unsigned int getComboBoxValueCount(void* this, unsigned int dwFieldID, unsigned int* pcItems, unsigned int* pdwSelectedItem);
unsigned int getComboBoxValueAt(void* this, unsigned int dwFieldID, unsigned int dwItem, void* ppszItem);
unsigned int setStringValue(void* this, unsigned int dwFieldID, void*  psz);
unsigned int setCheckboxValue(void* this, unsigned int dwFieldID, int bChecked);
unsigned int setComboBoxSelectedValue(void* this, unsigned int dwFieldID, unsigned int dwSelectedItem);
unsigned int commandLinkClicked(void* this, unsigned int dwFieldID);
unsigned int getSerialization(void* this, int *pcpgsr, void* pcpcs, void* ppszOptionalStatusText, int *pcpsiOptionalStatusIcon);
unsigned int reportResult(void* this, unsigned int status, unsigned int substatus, void** ppszOptionalStatusText, int *pcpsiOptionalStatusIcon);

extern void * hInstance;
*/
import "C"

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

//see mksyscall.go
//sys LsaConnectUntrusted(lsaHandle *windows.Handle) (err error) [failretval!=0] = Secur32.LsaConnectUntrusted
//sys LsaLookupAuthenticationPackage(lsaHandle windows.Handle, packageName *windows.NTString, authenticationPackage *uint32) (err error) [failretval!=0] = Secur32.LsaLookupAuthenticationPackage
//sys LsaDeregisterLogonProcess(lsaHandle windows.Handle) (err error) [failretval!=0] = Secur32.LsaDeregisterLogonProcess

//sys LoadImage(hInst unsafe.Pointer, name *uint16, imageType uint32, cx int32, cy int32, fuLoad uint32) (handle unsafe.Pointer, err error) [failretval==nil] = User32.LoadImageW

//go:embed "icon.bmp"
var icon []byte

const (
	IMAGE_BITMAP = 0
	LR_SHARED    = 0x00008000
)

// credentialProviderCredentialVTable is used for reimplementing C++ classes in Go. This is required for implementing
// // COM interfaces. We use method similar to this C implementation: https://www.codeproject.com/Articles/13601/COM-in-plain-C
var credentialProviderCredentialVTable = [20]unsafe.Pointer{
	C.queryInterface,
	C.addRef,
	C.release,
	C.credentialAdvise,
	C.unAdvise,
	C.setSelected,
	C.setDeselected,
	C.getFieldState,
	C.getStringValue,
	C.getBitmapValue,
	C.getCheckboxValue,
	C.getSubmitButtonValue,
	C.getComboBoxValueCount,
	C.getComboBoxValueAt,
	C.setStringValue,
	C.setCheckboxValue,
	C.setComboBoxSelectedValue,
	C.commandLinkClicked,
	C.getSerialization,
	C.reportResult,
}

//export credentialAdvise
func credentialAdvise(this unsafe.Pointer, pcpce unsafe.Pointer) uint32 {
	return S_OK
}

//export setSelected
func setSelected(this unsafe.Pointer, pbAutoLogon *int32) uint32 {
	return S_OK
}

//export setDeselected
func setDeselected(this unsafe.Pointer) uint32 {
	return S_OK
}

const (
	CPFIS_NONE           = 0
	CPFS_DISPLAY_IN_BOTH = 3
)

//export getFieldState
func getFieldState(this unsafe.Pointer, dwFieldID uint32, pcpfs *uint32, pcpfis *uint32) uint32 {
	*pcpfs = CPFS_DISPLAY_IN_BOTH
	*pcpfis = CPFIS_NONE
	return S_OK
}

//export getStringValue
func getStringValue(this unsafe.Pointer, dwFieldID uint32, ppsz unsafe.Pointer) uint32 {
	return E_NOTIMPL
}

//export getBitmapValue
func getBitmapValue(this unsafe.Pointer, dwFieldID uint32, phbmp *unsafe.Pointer) uint32 {
	name, err := windows.UTF16PtrFromString("LOGO")
	if err != nil {
		log.WithError(err).Error("can't convert logo name to UTF16")
		return E_FAIL
	}
	hBitmap, err := LoadImage(C.hInstance, name, IMAGE_BITMAP, 0, 0, LR_SHARED)
	if err != nil {
		log.WithError(err).Error("can't load tile")
		return E_FAIL
	}

	*phbmp = hBitmap

	return S_OK
}

//export getCheckboxValue
func getCheckboxValue(this unsafe.Pointer, dwFieldID uint32, pbChecked *int32, ppszLabel unsafe.Pointer) uint32 {
	return E_NOTIMPL
}

//export getSubmitButtonValue
func getSubmitButtonValue(this unsafe.Pointer, dwFieldID uint32, pdwAdjacentTo *uint32) uint32 {
	return E_NOTIMPL
}

//export getComboBoxValueCount
func getComboBoxValueCount(this unsafe.Pointer, dwFieldID uint32, pcItems *uint32, pdwSelectedItem *uint32) uint32 {
	return E_NOTIMPL
}

//export getComboBoxValueAt
func getComboBoxValueAt(this unsafe.Pointer, dwFieldID uint32, dwItem uint32, ppszItem unsafe.Pointer) uint32 {
	return E_NOTIMPL
}

//export setStringValue
func setStringValue(this unsafe.Pointer, dwFieldID uint32, psz unsafe.Pointer) uint32 {
	return E_NOTIMPL
}

//export setCheckboxValue
func setCheckboxValue(this unsafe.Pointer, dwFieldID uint32, bChecked int32) uint32 {
	return E_NOTIMPL
}

//export setComboBoxSelectedValue
func setComboBoxSelectedValue(this unsafe.Pointer, dwFieldID uint32, dwSelectedItem uint32) uint32 {
	return E_NOTIMPL
}

//export commandLinkClicked
func commandLinkClicked(this unsafe.Pointer, dwFieldID uint32) uint32 {
	return E_NOTIMPL
}

type CredentialSerialization struct {
	UlAuthenticationPackage uint32
	ClsidCredentialProvider windows.GUID
	CbSerialization         uint32
	RgbSerialization        *uint8
}

func getLsassHandle() (windows.Handle, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	if err := windows.Process32First(snap, &entry); err != nil {
		return 0, fmt.Errorf("Process32First failed: %w", err)
	}
	if windows.UTF16ToString(entry.ExeFile[:]) == "lsass.exe" {
		handle, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE, false, entry.ProcessID)
		if err != nil {
			err = fmt.Errorf("OpenProcess failed: %w", err)
		}
		return handle, err
	}
	for {
		if err := windows.Process32Next(snap, &entry); err != nil {
			return 0, fmt.Errorf("Process32Next failed: %w", err)
		}
		name := windows.UTF16ToString(entry.ExeFile[:])
		if name == "lsass.exe" {
			handle, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE, false, entry.ProcessID)
			if err != nil {
				err = fmt.Errorf("OpenProcess failed: %w", err)
			}
			return handle, err
		}
	}
}

func openAnonymousPipe() (windows.Handle, windows.Handle, windows.Handle, windows.Handle, error) {
	var rcph, waph, raph, wcph windows.Handle
	lsass, err := getLsassHandle()
	if err != nil {
		log.WithError(err).Error("can't find lsass handle")
		return 0, 0, 0, 0, err
	}
	defer windows.CloseHandle(lsass)

	if err := windows.CreatePipe(&rcph, &waph, nil, 256); err != nil {
		log.WithError(err).Error("can't open pipe")
		return 0, 0, 0, 0, err
	}
	if err := windows.CreatePipe(&raph, &wcph, nil, 256); err != nil {
		log.WithError(err).Error("can't open pipe")
		windows.CloseHandle(rcph)
		windows.CloseHandle(waph)
		return 0, 0, 0, 0, err
	}
	if err := windows.DuplicateHandle(windows.CurrentProcess(), waph, lsass, &waph, 0, false, windows.DUPLICATE_SAME_ACCESS|windows.DUPLICATE_CLOSE_SOURCE); err != nil {
		log.WithError(err).Error("can't duplicate handle")
		windows.CloseHandle(rcph)
		windows.CloseHandle(waph)
		windows.CloseHandle(raph)
		windows.CloseHandle(wcph)
		return 0, 0, 0, 0, err
	}
	if err := windows.DuplicateHandle(windows.CurrentProcess(), raph, lsass, &raph, 0, false, windows.DUPLICATE_SAME_ACCESS|windows.DUPLICATE_CLOSE_SOURCE); err != nil {
		log.WithError(err).Error("can't duplicate handle")
		windows.CloseHandle(rcph)
		windows.CloseHandle(waph)
		windows.CloseHandle(raph)
		windows.CloseHandle(wcph)
		return 0, 0, 0, 0, err
	}

	return rcph, wcph, raph, waph, nil
}

//export getSerialization
func getSerialization(this unsafe.Pointer, pcpgsr *int32, p unsafe.Pointer, ppszOptionalStatusText unsafe.Pointer, pcpsiOptionalStatusIcon *int32) uint32 {
	com := (*Com)(this)
	pcpcs := (*CredentialSerialization)(p)
	id, err := getPackageID(APName)
	if err != nil {
		log.WithError(err).Error("can't get package ID")
		return E_FAIL
	}
	*pcpgsr = 2
	pcpcs.UlAuthenticationPackage = id
	pcpcs.ClsidCredentialProvider = CredentialProviderGUID

	pin := string(com.data[:])

	rcph, wcph, raph, waph, err := openAnonymousPipe()
	if err != nil {
		log.WithError(err).Error("can't open pipe")
		return E_FAIL
	}

	go pipeSign(rcph, wcph, pin)

	certificate, err := Certificate()
	if err != nil {
		log.WithError(err).Error("can't get certificate")
		return E_FAIL
	}

	l := len(certificate.Raw) + 16
	pcpcs.CbSerialization = uint32(l)
	p, err = CoTaskMemAlloc(uint64(l))
	if err != nil {
		log.WithError(err).Error("can't allocate memory")
		return E_FAIL
	}
	pcpcs.RgbSerialization = (*byte)(p)
	serialization := unsafe.Slice(pcpcs.RgbSerialization, l)
	copy(serialization[16:], certificate.Raw)
	binary.LittleEndian.PutUint64(serialization, uint64(raph))
	binary.LittleEndian.PutUint64(serialization[8:], uint64(waph))

	return S_OK
}

//export reportResult
func reportResult(this unsafe.Pointer, status uint32, substatus uint32, ppszOptionalStatusText *unsafe.Pointer, pcpsiOptionalStatusIcon *int32) uint32 {
	if status == statusLogonFailure {
		switch substatus {
		case invalidLicense:
			s, err := coTaskMemString("This feature requires an enterprise license")
			if err != nil {
				return E_FAIL
			}
			*ppszOptionalStatusText = s
		case noSmartCardKeyUsage:
			s, err := coTaskMemString("Certificate can't be used for SmartCard login")
			if err != nil {
				return E_FAIL
			}
			*ppszOptionalStatusText = s
		case unknownError:
			s, err := coTaskMemString("Unknown error during authentication")
			if err != nil {
				return E_FAIL
			}
			*ppszOptionalStatusText = s
		}
	}
	return S_OK
}

func pipeSign(rh, wh windows.Handle, pin string) {
	defer windows.CloseHandle(rh)
	defer windows.CloseHandle(wh)
	rhf := os.NewFile(uintptr(rh), "rh")
	if rhf == nil {
		log.Error("can't open read pipe descriptor")
		return
	}
	data, err := io.ReadAll(rhf)
	if err != nil {
		log.WithError(err).Error("can't read challenge to sign")
	}
	sign, err := Sign(pin, data)
	if err != nil {
		log.WithError(err).Error("can't sign challenge")
		return
	}
	whf := os.NewFile(uintptr(wh), "wh")
	if whf == nil {
		log.Error("can't open write pipe descriptor")
		return
	}
	_, err = whf.Write(sign)
	if err != nil {
		log.WithError(err).Error("can't write signed challenge")
	}
}

func getPackageID(name string) (uint32, error) {
	var lsaHandle windows.Handle
	var id uint32

	if err := LsaConnectUntrusted(&lsaHandle); err != nil {
		log.WithError(err).Error("can't connect to LSA")
		return 0, err
	}
	defer LsaDeregisterLogonProcess(lsaHandle)

	s, err := windows.NewNTString(name)
	if err != nil {
		return 0, err
	}

	err = LsaLookupAuthenticationPackage(lsaHandle, s, &id)
	return id, err
}

// coTaskMemString converts provided string to UTF16 and copy it to the memory block allocated by CoTaskMemAlloc
// https://learn.microsoft.com/en-us/windows/win32/api/combaseapi/nf-combaseapi-cotaskmemalloc
func coTaskMemString(s string) (unsafe.Pointer, error) {
	us, err := windows.UTF16FromString(s)
	if err != nil {
		log.WithError(err).Error("can't convert string to UTF16")
		return nil, err
	}
	pp, err := CoTaskMemAlloc(uint64(len(us) * 2))
	if err != nil {
		log.WithError(err).Error("CoTaskMemAlloc failed")
		return nil, err
	}
	pps := unsafe.Slice((*uint16)(pp), len(us))
	copy(pps, us)
	return pp, nil
}
