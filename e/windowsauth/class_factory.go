package main

/*
#include "com.h"

unsigned int createInstance(void* this, void* outer, void* riid, void** ppv);
unsigned int lockServer(void* this, int lock);
*/
import "C"

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// classFactoryVTable is used for reimplementing C++ classes in Go. This is required for implementing
// COM interfaces. We use method similar to this C implementation: https://www.codeproject.com/Articles/13601/COM-in-plain-C
var classFactoryVTable = [5]unsafe.Pointer{C.queryInterface, C.addRef, C.release, C.createInstance, C.lockServer}

//export createInstance
func createInstance(_, outer unsafe.Pointer, riid unsafe.Pointer, ppv *unsafe.Pointer) uint32 {
	*ppv = nil
	if outer != nil {
		return uint32(windows.CLASS_E_NOAGGREGATION)
	}
	wriid := (*windows.GUID)(riid)
	if *wriid != ICredentialProviderGUID && *wriid != ICredentialProviderFilterGUID {
		return uint32(windows.E_NOINTERFACE)
	}

	*ppv = C.calloc(1, C.ulonglong(unsafe.Sizeof(Com{})))
	com := (*Com)(*ppv)
	com.count.Add(1)

	if *wriid == ICredentialProviderGUID {
		com.vTable = &credentialProviderVTable[0]
		com.guid = ICredentialProviderGUID
	} else {
		com.vTable = &credentialProviderFilterVTable[0]
		com.guid = ICredentialProviderFilterGUID
	}

	return S_OK
}

//export lockServer
func lockServer(_ unsafe.Pointer, lock int32) uint32 {
	return S_OK
}
