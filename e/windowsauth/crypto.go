package main

import (
	"crypto/sha1"
	"crypto/x509"
	"fmt"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

//see mksyscall.go
//sys CryptGetUserKey(provhandle windows.Handle, dwKeySpec uint32, phUserKey *windows.Handle) (err error) = advapi32.CryptGetUserKey
//sys CryptDestroyKey(provhandle windows.Handle) (err error) = advapi32.CryptDestroyKey
//sys CryptGetKeyParam(hKey windows.Handle, dwParam uint32, pbData *byte, pdwDataLen *uint32, dwFlags uint32) (err error) = advapi32.CryptGetKeyParam
//sys CryptSetProvParam(hProv windows.Handle, dwParam uint32, pbData *byte, dwFlags uint32) (err error) = advapi32.CryptSetProvParam

//sys CryptCreateHash(hProv windows.Handle, algId uint32, hKey windows.Handle, dwFlags uint32, phHash *windows.Handle) (err error) = advapi32.CryptCreateHash
//sys CryptDestroyHash(hHash windows.Handle) (err error) = advapi32.CryptDestroyHash
//sys CryptSignHash(hHash windows.Handle, dwKeySpec uint32, szDescription *uint16, dwFlags uint32, pbSignature *byte, pdwSigLen *uint32) (err error) = advapi32.CryptSignHashW
//sys CryptSetHashParam(hHash windows.Handle, dwParam uint32, pbData *byte, dwFlags uint32) (err error) = advapi32.CryptSetHashParam

const (
	CALG_SHA1          = 0x00008004
	KP_CERTIFICATE     = 26
	PP_KEYEXCHANGE_PIN = 0x20
	HP_HASHVAL         = 0x2
)

func wrap(format string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format, err)
}

func acquireContext() (windows.Handle, error) {
	var hprov windows.Handle
	container := windows.StringToUTF16Ptr("")
	csp := windows.StringToUTF16Ptr("Microsoft Base Smart Card Crypto Provider")
	err := windows.CryptAcquireContext(&hprov, container, csp, windows.AT_KEYEXCHANGE, windows.CRYPT_SILENT)
	return hprov, wrap("can't acquire context: %w", err)
}

func Certificate() (*x509.Certificate, error) {
	hprov, err := acquireContext()
	if err != nil {
		return nil, err
	}
	defer windows.CryptReleaseContext(hprov, 0)

	var hkey windows.Handle
	if err := CryptGetUserKey(hprov, windows.AT_KEYEXCHANGE, &hkey); err != nil {
		return nil, fmt.Errorf("can't get user key: %w", err)
	}
	defer CryptDestroyKey(hkey)

	var dataLength uint32
	if err = CryptGetKeyParam(hkey, KP_CERTIFICATE, nil, &dataLength, 0); err != nil {
		return nil, fmt.Errorf("can't get key param length: %w", err)
	}
	data := make([]byte, dataLength)
	if err = CryptGetKeyParam(hkey, KP_CERTIFICATE, &data[0], &dataLength, 0); err != nil {
		return nil, fmt.Errorf("can't get key param: %w", err)
	}

	return x509.ParseCertificate(data)
}

func Sign(pin string, data []byte) ([]byte, error) {
	log.Info("start of sign process")

	hprov, err := acquireContext()
	if err != nil {
		return nil, err
	}
	defer windows.CryptReleaseContext(hprov, 0)

	var hhash windows.Handle
	if err = CryptCreateHash(hprov, CALG_SHA1, windows.Handle(0), 0, &hhash); err != nil {
		return nil, wrap("can't create hash: %w", err)
	}
	defer CryptDestroyHash(hhash)

	sum := sha1.Sum(data)
	if err := CryptSetHashParam(hhash, HP_HASHVAL, &sum[0], 0); err != nil {
		return nil, wrap("can't set hash val: %w", err)
	}

	if err := setPin(pin, hprov); err != nil {
		return nil, err
	}

	var dataLength uint32
	if err := CryptSignHash(hhash, windows.AT_KEYEXCHANGE, nil, 0, nil, &dataLength); err != nil {
		return nil, wrap("can't sign hash: length: %w", err)
	}

	a := make([]byte, dataLength)
	if err := CryptSignHash(hhash, windows.AT_KEYEXCHANGE, nil, 0, &a[0], &dataLength); err != nil {
		return nil, wrap("can't sign hash: %w", err)
	}
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
	return a, nil
}

func setPin(pin string, hprov windows.Handle) error {
	bpin, err := windows.BytePtrFromString(pin)
	if err != nil {
		return wrap("can't set pin: %w", err)
	}
	return wrap("can't set pin: %w", CryptSetProvParam(hprov, PP_KEYEXCHANGE_PIN, bpin, 0))
}
