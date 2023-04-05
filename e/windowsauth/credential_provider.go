package main

/*
#include "com.h"

unsigned int setSerialization(void* this, void* pcpcs);
unsigned int advise(void* this, void* pcpe, void* upAdviseContext);
unsigned int unAdvise(void* this);
unsigned int getFieldDescriptorCount(void* this, unsigned int* pdwCount);
unsigned int getFieldDescriptorAt(void* this, unsigned int dwIndex, void** ppcpfd);
unsigned int getCredentialCount(void* this, unsigned int* pdwCount, unsigned int* pdwDefault, int* pbAutoLogonWithDefault);
unsigned int getCredentialAt(void* this, unsigned int dwIndex, void** ppcpc);
unsigned int setUsageScenario(void* this, int cpus, unsigned int dwFlags);
*/
import "C"

import (
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

// CredentialProviderGUID is GUID identifying this Credential Provider during COM registration
// See https://github.com/gravitational/teleport.e/blob/rfd/0002-non-ad-desktop-access/rfd/0002-non-ad-desktop-access.md#credential-provider
//
// Note: this must be kept in sync with the same value in installer/main.go
var CredentialProviderGUID = windows.GUID{
	Data1: 0xFF285315,
	Data2: 0x5335,
	Data3: 0x4F69,
	Data4: [8]byte{0xA9, 0xA2, 0x9C, 0xC5, 0xF8, 0x41, 0x9D, 0x55},
}

//see mksyscall.go
//sys CoTaskMemAlloc(cb uint64) (p unsafe.Pointer, err error) [failretval==nil] = ole32.CoTaskMemAlloc

// credentialProviderVTable is used for reimplementing C++ classes in Go. This is required for implementing
// COM interfaces. We use method similar to this C implementation: https://www.codeproject.com/Articles/13601/COM-in-plain-C
var credentialProviderVTable = [11]unsafe.Pointer{
	C.queryInterface,
	C.addRef,
	C.release,
	C.setUsageScenario,
	C.setSerialization,
	C.advise,
	C.unAdvise,
	C.getFieldDescriptorCount,
	C.getFieldDescriptorAt,
	C.getCredentialCount,
	C.getCredentialAt,
}

//export setUsageScenario
func setUsageScenario(this unsafe.Pointer, cpus int32, dwFlags uint32) uint32 {
	return S_OK
}

//export setSerialization
func setSerialization(this unsafe.Pointer, pcpcs unsafe.Pointer) uint32 {
	s := (*CredentialSerialization)(pcpcs)
	if s.CbSerialization != 9 || *s.RgbSerialization != 0xAA {
		log.WithFields(log.Fields{
			"type":   *s.RgbSerialization,
			"length": s.CbSerialization,
		}).Error("unexpected data")
		return E_UNEXPECTED
	}
	com := (*Com)(this)
	copy(com.data[:], unsafe.Slice(s.RgbSerialization, 9)[1:])
	return S_OK
}

//export advise
func advise(this unsafe.Pointer, pcpe unsafe.Pointer, upAdviseContext unsafe.Pointer) uint32 {
	return S_OK
}

//export unAdvise
func unAdvise(this unsafe.Pointer) uint32 {
	return S_OK
}

//export getFieldDescriptorCount
func getFieldDescriptorCount(this unsafe.Pointer, pdwCount *uint32) uint32 {
	*pdwCount = 1
	return S_OK
}

type FieldDescriptor struct {
	DwFieldID     uint32
	Cpft          uint32
	PszLabel      *uint16
	GuidFieldType windows.GUID
}

//export getFieldDescriptorAt
func getFieldDescriptorAt(this unsafe.Pointer, dwIndex uint32, ppcpfd *unsafe.Pointer) uint32 {
	p, err := CoTaskMemAlloc(uint64(unsafe.Sizeof(FieldDescriptor{})))
	if err != nil {
		log.WithError(err).Error("can't allocate FieldDescriptor")
		return uint32(windows.E_OUTOFMEMORY)
	}

	iconDescriptor := (*FieldDescriptor)(p)
	iconDescriptor.DwFieldID = 0
	iconDescriptor.Cpft = 6
	iconDescriptor.PszLabel = nil

	*ppcpfd = p

	return S_OK
}

//export getCredentialCount
func getCredentialCount(this unsafe.Pointer, pdwCount *uint32, pdwDefault *uint32, pbAutoLogonWithDefault *int32) uint32 {
	com := (*Com)(this)
	if com.data[0] == 0 {
		log.Warn("no PIN available")
		*pdwCount = 0
		return S_OK
	}
	*pdwCount = 1
	*pbAutoLogonWithDefault = 1
	*pdwDefault = 0
	return S_OK
}

//export getCredentialAt
func getCredentialAt(this unsafe.Pointer, dwIndex uint32, ppcpc *unsafe.Pointer) uint32 {
	cp := (*Com)(this)
	*ppcpc = C.calloc(1, C.ulonglong(unsafe.Sizeof(Com{})))
	com := (*Com)(*ppcpc)
	com.vTable = &credentialProviderCredentialVTable[0]
	com.guid = ICredentialProviderCredentialGUID
	com.count.Add(1)
	com.data = cp.data
	return S_OK
}
