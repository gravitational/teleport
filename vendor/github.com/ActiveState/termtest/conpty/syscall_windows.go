// Copyright 2020 ActiveState Software. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

package conpty

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// load some windows system procedures

var (
	kernel32                              = windows.NewLazySystemDLL("kernel32.dll")
	procResizePseudoConsole               = kernel32.NewProc("ResizePseudoConsole")
	procCreatePseudoConsole               = kernel32.NewProc("CreatePseudoConsole")
	procClosePseudoConsole                = kernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttributeList = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute         = kernel32.NewProc("UpdateProcThreadAttribute")
	procLocalAlloc                        = kernel32.NewProc("LocalAlloc")
	procDeleteProcThreadAttributeList     = kernel32.NewProc("DeleteProcThreadAttributeList")
	procCreateProcessW                    = kernel32.NewProc("CreateProcessW")
)

// an extended version of a process startup info, the attribute list points
// to a pseudo terminal object
type startupInfoEx struct {
	startupInfo     windows.StartupInfo
	lpAttributeList windows.Handle
}

// constant used in CreateProcessW indicating that extended startup info is present
const extendedStartupinfoPresent uint32 = 0x00080000

type procThreadAttribute uintptr

// windows constant needed during initialization of extended startupinfo
const procThreadAttributePseudoconsole procThreadAttribute = 22 | 0x00020000 // this is the only one we support right now

func initializeProcThreadAttributeList(attributeList uintptr, attributeCount uint32, listSize *uint64) (err error) {

	if attributeList == 0 {
		procInitializeProcThreadAttributeList.Call(0, uintptr(attributeCount), 0, uintptr(unsafe.Pointer(listSize)))
		return
	}
	r1, _, e1 := procInitializeProcThreadAttributeList.Call(attributeList, uintptr(attributeCount), 0, uintptr(unsafe.Pointer(listSize)))

	if r1 == 0 { // boolean FALSE
		err = e1
	}

	return
}

func updateProcThreadAttributeList(attributeList windows.Handle, attribute procThreadAttribute, lpValue windows.Handle, lpSize uintptr) (err error) {

	r1, _, e1 := procUpdateProcThreadAttribute.Call(uintptr(attributeList), 0, uintptr(attribute), uintptr(lpValue), lpSize, 0, 0)

	if r1 == 0 { // boolean FALSE
		err = e1
	}

	return
}
func deleteProcThreadAttributeList(handle windows.Handle) (err error) {
	r1, _, e1 := procDeleteProcThreadAttributeList.Call(uintptr(handle))

	if r1 == 0 { // boolean FALSE
		err = e1
	}

	return
}

func localAlloc(size uint64) (ptr windows.Handle, err error) {
	r1, _, e1 := procLocalAlloc.Call(uintptr(0x0040), uintptr(size))
	if r1 == 0 {
		err = e1
		ptr = windows.InvalidHandle
		return
	}
	ptr = windows.Handle(r1)
	return
}

func createPseudoConsole(consoleSize uintptr, ptyIn windows.Handle, ptyOut windows.Handle, hpCon *windows.Handle) (err error) {
	r1, _, e1 := procCreatePseudoConsole.Call(consoleSize, uintptr(ptyIn), uintptr(ptyOut), 0, uintptr(unsafe.Pointer(hpCon)))

	if r1 != 0 { // !S_OK
		err = e1
	}
	return
}

func resizePseudoConsole(handle windows.Handle, consoleSize uintptr) (err error) {
	r1, _, e1 := procResizePseudoConsole.Call(uintptr(handle), consoleSize)
	if r1 != 0 { // !S_OK
		err = e1
	}
	return
}

func closePseudoConsole(handle windows.Handle) (err error) {
	r1, _, e1 := procClosePseudoConsole.Call(uintptr(handle))
	if r1 == 0 {
		err = e1
	}

	return
}
