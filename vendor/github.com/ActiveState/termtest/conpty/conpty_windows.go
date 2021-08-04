// Copyright 2020 ActiveState Software. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

package conpty

import (
	"fmt"
	"log"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ConPty represents a windows pseudo console.
// Attach a process to it by calling the Spawn() method.
// You can send UTF encoded commands to it with Write() and listen to
// its output stream by accessing the output pipe via OutPipe()
type ConPty struct {
	hpCon               *windows.Handle
	pipeFdIn            windows.Handle
	pipeFdOut           windows.Handle
	startupInfo         startupInfoEx
	consoleSize         uintptr
	inPipe              *os.File
	outPipe             *os.File
	attributeListBuffer []byte
}

// New returns a new ConPty pseudo terminal device
func New(columns int16, rows int16) (c *ConPty, err error) {
	c = &ConPty{
		hpCon:       new(windows.Handle),
		startupInfo: startupInfoEx{},
		consoleSize: uintptr(columns) + (uintptr(rows) << 16),
	}
	err = c.createPseudoConsoleAndPipes()
	if err != nil {
		return nil, err
	}
	err = c.initializeStartupInfoAttachedToPTY()
	if err != nil {
		return nil, err
	}
	return
}

// Close closes the pseudo-terminal and cleans up all attached resources
func (c *ConPty) Close() (err error) {
	err = deleteProcThreadAttributeList(c.startupInfo.lpAttributeList)
	if err != nil {
		log.Printf("Failed to free delete proc thread attribute list: %v", err)
	}
	/*
		_, err = windows.LocalFree(c.startupInfo.lpAttributeList)
		if err != nil {
			log.Printf("Failed to free the lpAttributeList")
		}
	*/
	err = closePseudoConsole(*c.hpCon)
	if err != nil {
		log.Printf("Failed to close pseudo console: %v", err)
	}
	c.inPipe.Close()
	c.outPipe.Close()
	return
}

// OutPipe returns the output pipe of the pseudo terminal
func (c *ConPty) OutPipe() *os.File {
	return c.outPipe
}

// InPipe returns input pipe of the pseudo terminal
// Note: It is safer to use the Write method to prevent partially-written VT sequences
// from corrupting the terminal
func (c *ConPty) InPipe() *os.File {
	return c.inPipe
}

func (c *ConPty) OutFd() uintptr {
	return c.outPipe.Fd()
}

// Write safely writes bytes to the pseudo terminal
func (c *ConPty) Write(buf []byte) (uint32, error) {
	var n uint32
	err := windows.WriteFile(c.pipeFdIn, buf, &n, nil)
	return n, err
}

var zeroProcAttr syscall.ProcAttr

// Spawn spawns a new process attached to the pseudo terminal
func (c *ConPty) Spawn(argv0 string, argv []string, attr *syscall.ProcAttr) (pid int, handle uintptr, err error) {

	if attr == nil {
		attr = &zeroProcAttr
	}

	if attr.Sys != nil {
		log.Printf("Warning: SysProc attributes are not supported by Spawn.")
	}

	if len(attr.Files) != 0 {
		log.Printf("Warning: Ignoring 'Files' attribute in ProcAttr argument.")
	}

	if len(attr.Dir) != 0 {
		// StartProcess assumes that argv0 is relative to attr.Dir,
		// because it implies Chdir(attr.Dir) before executing argv0.
		// Windows CreateProcess assumes the opposite: it looks for
		// argv0 relative to the current directory, and, only once the new
		// process is started, it does Chdir(attr.Dir). We are adjusting
		// for that difference here by making argv0 absolute.
		var err error
		argv0, err = joinExeDirAndFName(attr.Dir, argv0)
		if err != nil {
			return 0, 0, err
		}
	}
	argv0p, err := windows.UTF16PtrFromString(argv0)
	if err != nil {
		return 0, 0, err
	}

	// Windows CreateProcess takes the command line as a single string:
	// use attr.CmdLine if set, else build the command line by escaping
	// and joining each argument with spaces
	cmdline := makeCmdLine(argv)

	var argvp *uint16
	if len(cmdline) != 0 {
		argvp, err = windows.UTF16PtrFromString(cmdline)
		if err != nil {
			return 0, 0, err
		}
	}

	var dirp *uint16
	if len(attr.Dir) != 0 {
		dirp, err = windows.UTF16PtrFromString(attr.Dir)
		if err != nil {
			return 0, 0, err
		}
	}

	c.startupInfo.startupInfo.Flags = windows.STARTF_USESTDHANDLES

	pi := new(windows.ProcessInformation)

	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT) | extendedStartupinfoPresent

	var zeroSec windows.SecurityAttributes
	pSec := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(zeroSec)), InheritHandle: 1}
	tSec := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(zeroSec)), InheritHandle: 1}

	// c.startupInfo.startupInfo.Cb = uint32(unsafe.Sizeof(c.startupInfo))
	err = windows.CreateProcess(
		argv0p,
		argvp,
		pSec, // process handle not inheritable
		tSec, // thread handles not inheritable,
		false,
		flags,
		createEnvBlock(addCriticalEnv(dedupEnvCase(true, attr.Env))),
		dirp, // use current directory later: dirp,
		&c.startupInfo.startupInfo,
		pi)

	if err != nil {
		return 0, 0, err
	}
	defer windows.CloseHandle(windows.Handle(pi.Thread))

	return int(pi.ProcessId), uintptr(pi.Process), nil
}

func (c *ConPty) createPseudoConsoleAndPipes() (err error) {
	var hPipePTYIn windows.Handle
	var hPipePTYOut windows.Handle

	if err := windows.CreatePipe(&hPipePTYIn, &c.pipeFdIn, nil, 0); err != nil {
		log.Fatalf("Failed to create PTY input pipe: %v", err)
	}
	if err := windows.CreatePipe(&c.pipeFdOut, &hPipePTYOut, nil, 0); err != nil {
		log.Fatalf("Failed to create PTY output pipe: %v", err)
	}

	err = createPseudoConsole(c.consoleSize, hPipePTYIn, hPipePTYOut, c.hpCon)
	if err != nil {
		return fmt.Errorf("failed to create pseudo console: %d, %v", uintptr(*c.hpCon), err)
	}

	// Note: We can close the handles to the PTY-end of the pipes here
	// because the handles are dup'ed into the ConHost and will be released
	// when the ConPTY is destroyed.
	if hPipePTYOut != windows.InvalidHandle {
		windows.CloseHandle(hPipePTYOut)
	}
	if hPipePTYIn != windows.InvalidHandle {
		windows.CloseHandle(hPipePTYIn)
	}

	c.inPipe = os.NewFile(uintptr(c.pipeFdIn), "|0")
	c.outPipe = os.NewFile(uintptr(c.pipeFdOut), "|1")

	return
}

func (c *ConPty) Resize(cols uint16, rows uint16) error {
	return resizePseudoConsole(*c.hpCon, uintptr(cols)+(uintptr(rows)<<16))
}

func (c *ConPty) initializeStartupInfoAttachedToPTY() (err error) {

	var attrListSize uint64
	c.startupInfo.startupInfo.Cb = uint32(unsafe.Sizeof(c.startupInfo))

	err = initializeProcThreadAttributeList(0, 1, &attrListSize)
	if err != nil {
		return fmt.Errorf("could not retrieve list size: %v", err)
	}

	c.attributeListBuffer = make([]byte, attrListSize)
	// c.startupInfo.lpAttributeList, err = localAlloc(attrListSize)
	// if err != nil {
	//	return fmt.Errorf("Could not allocate local memory: %v", err)
	// }

	c.startupInfo.lpAttributeList = windows.Handle(unsafe.Pointer(&c.attributeListBuffer[0]))

	err = initializeProcThreadAttributeList(uintptr(c.startupInfo.lpAttributeList), 1, &attrListSize)
	if err != nil {
		return fmt.Errorf("failed to initialize proc thread attributes for conpty: %v", err)
	}

	err = updateProcThreadAttributeList(
		c.startupInfo.lpAttributeList,
		procThreadAttributePseudoconsole,
		*c.hpCon,
		unsafe.Sizeof(*c.hpCon))
	if err != nil {
		return fmt.Errorf("failed to update proc thread attributes attributes for conpty usage: %v", err)
	}

	return
}
