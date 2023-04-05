package main

/*
#include "com.h"

unsigned int filter(void* this, unsigned int cpus, unsigned int dwFlags, void* rgclsidProviders, void* rgbAllow, unsigned int cProviders);
unsigned int updateRemoteCredential(void* this, void* pcpcsIn, void* pcpcsOut);
*/
import "C"

import (
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var credentialProviderFilterVTable = [5]unsafe.Pointer{C.queryInterface, C.addRef, C.release, C.filter, C.updateRemoteCredential}

//export filter
func filter(this unsafe.Pointer, cpus, dwFlags uint32, rgclsidProviders, rgbAllow unsafe.Pointer, cProviders uint32) uint32 {
	return S_OK
}

type KerbCertificateLogin struct {
	MessageType   uint32
	DomainName    windows.NTUnicodeString
	UserName      windows.NTUnicodeString
	Pin           windows.NTUnicodeString
	Flags         uint32
	CspDataLength uint32
	CspData       *uint8
}

//export updateRemoteCredential
func updateRemoteCredential(this, pcpcsIn, pcpcsOut unsafe.Pointer) uint32 {
	in := (*CredentialSerialization)(pcpcsIn)
	out := (*CredentialSerialization)(pcpcsOut)
	kerb := (*KerbCertificateLogin)(unsafe.Pointer(in.RgbSerialization))

	if in.CbSerialization == 0 {
		log.Error("no serialization")
		*out = *in
		return E_FAIL
	}
	if *in.RgbSerialization != 0x0D {
		log.WithField("type", *in.RgbSerialization).Error("unknown message type")
		*out = *in
		return E_FAIL
	}

	pinu := kerb.Pin
	pinu.Buffer = (*uint16)(unsafe.Add(unsafe.Pointer(kerb), uintptr(unsafe.Pointer(kerb.Pin.Buffer))))
	pin := pinu.String()
	out.CbSerialization = uint32(len(pin) + 1)
	out.ClsidCredentialProvider = CredentialProviderGUID
	p, err := CoTaskMemAlloc(uint64(out.CbSerialization))
	if err != nil {
		log.WithError(err).Error("can't allocate serialization buffer")
		*out = *in
		return E_FAIL
	}
	out.RgbSerialization = (*uint8)(p)

	log.Info("valid PIN found, redirecting to Teleport credential provider")

	outs := unsafe.Slice(out.RgbSerialization, out.CbSerialization)
	copy(outs[1:], pin)
	outs[0] = 0xAA

	return S_OK
}
