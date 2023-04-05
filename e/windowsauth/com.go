package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	S_OK         = uint32(windows.S_OK)
	E_NOTIMPL    = uint32(windows.E_NOTIMPL)
	E_FAIL       = uint32(windows.E_FAIL)
	E_UNEXPECTED = uint32(windows.E_UNEXPECTED)
)

// GUIDs required for different COM interfaces

// IUnknownGUID see https://learn.microsoft.com/en-us/windows/win32/api/unknwn/nn-unknwn-iunknown
// This value is copied from unknwn.h from MinGW
var IUnknownGUID = windows.GUID{Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}

// ICredentialProviderGUID see https://learn.microsoft.com/en-us/windows/win32/api/credentialprovider/nn-credentialprovider-icredentialprovider
// This value is copied from credentialprovider.h from MinGW
var ICredentialProviderGUID = windows.GUID{
	Data1: 0xd27c3481,
	Data2: 0x5a1c,
	Data3: 0x45b2,
	Data4: [8]byte{0x8a, 0xaa, 0xc2, 0x0e, 0xbb, 0xe8, 0x22, 0x9e},
}

// ICredentialProviderCredentialGUID see https://learn.microsoft.com/en-us/windows/win32/api/credentialprovider/nn-credentialprovider-icredentialprovidercredential
// This value is copied from credentialprovider.h from MinGW
var ICredentialProviderCredentialGUID = windows.GUID{
	Data1: 0x63913a93,
	Data2: 0x40c1,
	Data3: 0x481a,
	Data4: [8]byte{0x81, 0x8d, 0x40, 0x72, 0xff, 0x8c, 0x70, 0xcc},
}

// ICredentialProviderFilterGUID see https://learn.microsoft.com/en-us/windows/win32/api/credentialprovider/nn-credentialprovider-icredentialproviderfilter
// This value is copied from credentialprovider.h from MinGW
var ICredentialProviderFilterGUID = windows.GUID{
	Data1: 0xa5da53f9,
	Data2: 0xd475,
	Data3: 0x4080,
	Data4: [8]byte{0xa1, 0x20, 0x91, 0x0c, 0x4a, 0x73, 0x98, 0x80},
}

// IClassFactoryGUID see https://learn.microsoft.com/en-us/windows/win32/api/unknwn/nn-unknwn-iclassfactory
// This value is copied from unknwn.h from MinGW
var IClassFactoryGUID = windows.GUID{
	Data1: 0x00000001,
	Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

// Com is generic COM object that can implement various COM interfaces using vTable.
// Do not change order of fields in this struct!
// vTable has to be pointer to first element of array of C functions. If you want to pass a Go function you have export it
// and create forward declaration in import "C" section.
type Com struct {
	vTable *unsafe.Pointer
	guid   windows.GUID
	data   [8]byte
	count  atomic.Int32
}

//export queryInterface
func queryInterface(p unsafe.Pointer, riid unsafe.Pointer, ppv *unsafe.Pointer) uint32 {
	this := (*Com)(p)
	wriid := (*windows.GUID)(riid)
	if *wriid == this.guid || *wriid == IUnknownGUID {
		addRef(p)
		*ppv = p
		return S_OK
	}
	*ppv = nil
	return uint32(windows.E_NOINTERFACE)
}

//export addRef
func addRef(p unsafe.Pointer) int32 {
	this := (*Com)(p)
	return this.count.Add(1)
}

//export release
func release(p unsafe.Pointer) int32 {
	this := (*Com)(p)
	count := this.count.Add(-1)
	if count == 0 {
		C.free(p)
	}
	return count
}
