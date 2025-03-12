//go:build pam && cgo
// +build pam,cgo

/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package pam

// #cgo LDFLAGS: -ldl
// #include <stdio.h>
// #include <stdlib.h>
// #include <string.h>
// #include <unistd.h>
// #include <dlfcn.h>
// #include <security/pam_appl.h>
// extern char *library_name();
// extern char* readCallback(int, int);
// extern void writeCallback(int n, int s, char* c);
// extern struct pam_conv *make_pam_conv(int);
// extern int _pam_start(void *, const char *, const char *, const struct pam_conv *, pam_handle_t **);
// extern int _pam_putenv(void *, pam_handle_t *, const char *);
// extern int _pam_end(void *, pam_handle_t *, int);
// extern int _pam_authenticate(void *, pam_handle_t *, int);
// extern int _pam_acct_mgmt(void *, pam_handle_t *, int);
// extern int _pam_open_session(void *, pam_handle_t *, int);
// extern int _pam_close_session(void *, pam_handle_t *, int);
// extern char **_pam_getenvlist(void *handle, pam_handle_t *pamh);
// extern const char *_pam_strerror(void *, pam_handle_t *, int);
// extern int _pam_envlist_len(char **);
// extern char * _pam_getenv(char **, int);
import "C"

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func init() {
	// Lock all PAM commands to the startup thread. From LockOSThread docs:
	//
	//     All init functions are run on the startup thread. Calling LockOSThread from
	//     an init function will cause the main function to be invoked on that thread.
	//
	// This is needed for pam_loginuid.so. It writes "/proc/self/loginuid"
	// which, on Linux, depends on being called from a specific thread.  If
	// it's not running on the right thread, pam_loginuid.so may fail with
	// EPERM sporadically.
	//
	// > Why the startup thread specifically?
	//   The kernel does some validation based on the thread context. I could
	//   not find what the kernel uses specifically. Some relevant code:
	//   https://github.com/torvalds/linux/blob/9d99b1647fa56805c1cfef2d81ee7b9855359b62/kernel/audit.c#L2284-L2317
	//   Locking to the startup thread seems to make the kernel happy.
	//   If you figure out more, please update this comment.
	//
	// > Why not call LockOSThread from pam.Open?
	//   By the time pam.Open gets called, more goroutines could've been
	//   spawned.  This means that the main goroutine (running pam.Open) could
	//   get re-scheduled to a different thread.
	//
	// > Why does pam.Open run on the main goroutine?
	//   This is an assumption. As of today, this is true because teleport
	//   re-executes itself and calls pam.Open synchronously. If we change this
	//   later, loginuid can become flaky again.
	//
	// > What does OpenSSH do?
	//   OpenSSH has a separate "authentication thread" which does all the PAM
	//   stuff:
	//   https://github.com/openssh/openssh-portable/blob/598c3a5e3885080ced0d7c40fde00f1d5cdbb32b/auth-pam.c#L470-L474
	//
	// Some historic context:
	// https://github.com/gravitational/teleport/issues/2476
	runtime.LockOSThread()
}

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentPAM)

const (
	// maxMessageSize is the maximum size of a message to accept from PAM. In
	// theory it should be PAM_MAX_MSG_SIZE which is defined as the "max size (in
	// chars) of each messages passed to the application through the conversation
	// function call". [1] Unfortunately this does not appear respected by OpenSSH,
	// which takes any size message and writes it out. [2]
	//
	// However, rather than accepting a message of unknown size from C code,
	// increase the maximum size to about 1 MB. This will allow Teleport to
	// print even long MOTD messages while not allowing C code to allocate
	// unbound memory in Go.
	//
	// [1] http://pubs.opengroup.org/onlinepubs/008329799/apdxa.htm
	// [2] https://github.com/openssh/openssh-portable/blob/V_8_0/auth-pam.c#L615-L654
	maxMessageSize = 2000 * C.PAM_MAX_MSG_SIZE

	// maxEnvironmentVariableSize is the maximum size of any environment
	// variable. Even though pam_env.so sets this to a maximum of 1024, set
	// it to be 10x that because not all PAM modules follow that convention. [1]
	//
	// [1] https://github.com/linux-pam/linux-pam/blob/master/modules/pam_env/pam_env.c#L55
	maxEnvironmentVariableSize = 1024 * 10
)

// handler is used to register and find instances of *PAM at the package level
// to enable callbacks from C code.
type handler interface {
	// writeStream will write to the output stream (stdout or stderr or
	// equivalent).
	writeStream(int, string) (int, error)

	// readStream will read from the input stream (stdin or equivalent).
	readStream(bool) (string, error)
}

var handlerMu sync.Mutex
var handlerCount int
var handlers map[int]handler = make(map[int]handler)

//export writeCallback
func writeCallback(index C.int, stream C.int, s *C.char) {
	handle, err := lookupHandler(int(index))
	if err != nil {
		logger.ErrorContext(context.Background(), "Unable to write to output stream", "error", err)
		return
	}

	// Convert C string to a Go string with a max size of maxMessageSize
	// (about 1 MB).
	str := C.GoStringN(s, C.int(C.strnlen(s, C.size_t(maxMessageSize))))

	// Write to the stream (typically stdout or stderr or equivalent).
	handle.writeStream(int(stream), str)
}

//export readCallback
func readCallback(index C.int, e C.int) *C.char {
	handle, err := lookupHandler(int(index))
	if err != nil {
		logger.ErrorContext(context.Background(), "Unable to read from input stream", "error", err)
		return nil
	}

	var echo bool
	if e == 1 {
		echo = true
	}

	// Read from the stream (typically stdin or equivalent).
	s, err := handle.readStream(echo)
	if err != nil {
		logger.ErrorContext(context.Background(), "Unable to read from input stream", "error", err)
		return nil
	}

	// Return one less than PAM_MAX_RESP_SIZE to prevent a Teleport user from
	// sending more than a PAM module can handle and to allow space for \0.
	//
	// Note: The function C.CString allocates memory using malloc. The memory is
	// not released in Go code because the caller of the callback function (PAM
	// module) will release it. C.CString will null terminate s.
	n := int(C.PAM_MAX_RESP_SIZE)
	if len(s) > n-1 {
		return C.CString(s[:n-1])
	}
	return C.CString(s)
}

// registerHandler will register a instance of *PAM with the package level
// handlers to support callbacks from C.
func registerHandler(p *PAM) int {
	handlerMu.Lock()
	defer handlerMu.Unlock()

	// The make_pam_conv function allocates struct pam_conv on the heap. It will
	// be released by Close function.
	handlerCount = handlerCount + 1
	p.conv = C.make_pam_conv(C.int(handlerCount))
	handlers[handlerCount] = p

	return handlerCount
}

// unregisterHandler will remove the PAM handle from the package level map
// once no more C callbacks can come back.
func unregisterHandler(handlerIndex int) {
	handlerMu.Lock()
	defer handlerMu.Unlock()

	delete(handlers, handlerIndex)
}

// lookupHandler returns a particular handler from the package level map.
func lookupHandler(handlerIndex int) (handler, error) {
	handlerMu.Lock()
	defer handlerMu.Unlock()

	handle, ok := handlers[handlerIndex]
	if !ok {
		return nil, trace.BadParameter("handler with index %v not registered", handlerIndex)
	}

	return handle, nil
}

var buildHasPAM bool = true
var systemHasPAM bool = false

// pamHandle is a opaque handle to the libpam object.
var pamHandle unsafe.Pointer

func init() {
	// Obtain a handle to the PAM library at runtime. The package level variable
	// SystemHasPAM is updated to true if a handle is obtained.
	//
	// Note: Since this handle is needed the entire time Teleport runs, dlclose()
	// is never called. The OS will cleanup when the process exits.
	pamHandle = C.dlopen(C.library_name(), C.RTLD_NOW)
	if pamHandle != nil {
		systemHasPAM = true
	}
}

// PAM is used to create a PAM context and initiate PAM transactions to check
// the users account and open/close a session.
type PAM struct {
	// pamh is a handle to the PAM transaction state.
	pamh *C.pam_handle_t

	// conv is the PAM conversation function for communication between
	// Teleport and the PAM module.
	conv *C.struct_pam_conv

	// retval holds the value returned by the last PAM call.
	retval C.int

	// stdin is the input stream which the conversation function will use to
	// obtain data from the user.
	stdin io.Reader

	// stdout is the output stream which the conversation function will use to
	// show data to the user.
	stdout io.Writer

	// stderr is the output stream which the conversation function will use to
	// report errors to the user.
	stderr io.Writer

	// service_name is the name of the PAM policy to use.
	service_name *C.char

	// login is the *nix login that that is being used.
	login *C.char

	// handlerIndex is the index to the package level handler map.
	handlerIndex int

	// once is used to ensure that any memory allocated by C only has free
	// called on it once.
	once sync.Once
}

// Open creates a PAM context and initiates a PAM transaction to check the
// account and then opens a session.
func Open(config *servicecfg.PAMConfig) (*PAM, error) {
	if config == nil {
		return nil, trace.BadParameter("PAM configuration is required.")
	}
	err := config.CheckDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p := &PAM{
		pamh:   nil,
		stdin:  config.Stdin,
		stdout: config.Stdout,
		stderr: config.Stderr,
	}

	// Both config.ServiceName and config.Username convert between Go strings to
	// C strings. Since the C strings are allocated on the heap in Go code, this
	// memory must be released (and will be on the call to the Close method).
	p.service_name = C.CString(config.ServiceName)
	p.login = C.CString(config.Login)

	// C code does not know that this PAM context exists. To ensure the
	// conversation function can get messages to the right context, a handle
	// registry at the package level is created (handlers). Each instance of the
	// PAM context has it's own handle which is used to communicate between C
	// and a instance of a PAM context.
	p.handlerIndex = registerHandler(p)

	// Create and initialize a PAM context. The pam_start function will
	// allocate pamh if needed and the pam_end function will release any
	// allocated memory.
	p.retval = C._pam_start(pamHandle, p.service_name, p.login, p.conv, &p.pamh)
	if p.retval != C.PAM_SUCCESS {
		return nil, p.codeToError(p.retval)
	}

	for k, v := range config.Env {
		// Set a regular OS env var on this process which should be available
		// to child PAM processes.
		os.Setenv(k, v)

		// Also set it via PAM-specific pam_putenv, which is respected by
		// pam_exec (and possibly others), where parent env vars are not.
		kv := C.CString(fmt.Sprintf("%s=%s", k, v))
		// pam_putenv makes a copy of kv, so we can free it right away.
		defer C.free(unsafe.Pointer(kv))
		retval := C._pam_putenv(pamHandle, p.pamh, kv)
		if retval != C.PAM_SUCCESS {
			return nil, p.codeToError(retval)
		}
	}

	// Trigger the "account" PAM hooks for this login.
	//
	// Check that the *nix account is valid. Checking an account varies based off
	// the PAM modules used in the account stack. Typically this consists of
	// checking if the account is expired or has access restrictions.
	//
	// Note: This function does not perform any authentication!
	retval := C._pam_acct_mgmt(pamHandle, p.pamh, 0)
	if retval != C.PAM_SUCCESS {
		return nil, p.codeToError(retval)
	}

	if config.UsePAMAuth {
		// Trigger the "auth" PAM hooks for this login.
		//
		// These would perform any extra authentication steps configured in the PAM
		// stack, like per-session 2FA.
		retval = C._pam_authenticate(pamHandle, p.pamh, 0)
		if retval != C.PAM_SUCCESS {
			return nil, p.codeToError(retval)
		}
	}

	// Trigger the "session" PAM hooks for this login.
	//
	// Open a user session. Opening a session varies based off the PAM modules
	// used in the "session" stack. Opening a session typically consists of
	// printing the MOTD, mounting a home directory, updating auth.log.
	p.retval = C._pam_open_session(pamHandle, p.pamh, 0)
	if p.retval != C.PAM_SUCCESS {
		return nil, p.codeToError(p.retval)
	}

	return p, nil
}

// Close will close the session, the PAM context, and release any allocated
// memory.
func (p *PAM) Close() error {
	// Close the PAM session. Closing a session can entail anything from
	// unmounting a home directory and updating auth.log.
	p.retval = C._pam_close_session(pamHandle, p.pamh, 0)
	if p.retval != C.PAM_SUCCESS {
		return p.codeToError(p.retval)
	}

	// Unregister handler index at the package level.
	unregisterHandler(p.handlerIndex)

	// Free any allocated memory on close.
	p.free()

	return nil
}

// Environment returns the PAM environment variables associated with a PAM
// handle. For example pam_env.so reads in certain environment variables
// which then have to be set when spawning the users shell.
//
// Note that pam_getenvlist is used to fetch the list of PAM environment
// variables and it is the responsibility of the caller to free that memory.
// From http://man7.org/linux/man-pages/man3/pam_getenvlist.3.html:
//
//	It should be noted that this memory will never be free()'d by libpam.
//	Once obtained by a call to pam_getenvlist, it is the responsibility
//	of the calling application to free() this memory.
func (p *PAM) Environment() []string {
	// Get list of additional environment variables requested from PAM.
	pam_envlist := C._pam_getenvlist(pamHandle, p.pamh)
	defer C.free(unsafe.Pointer(pam_envlist))

	// Find out how many environment variables exist and size the output
	// slice. This is pushed to C to avoid doing pointer arithmetic in Go.
	n := int(C._pam_envlist_len(pam_envlist))
	env := make([]string, 0, n)

	// Loop over all environment variables and convert them to a Go string.
	for i := 0; i < n; i++ {
		pam_env := C._pam_getenv(pam_envlist, C.int(i))
		defer C.free(unsafe.Pointer(pam_env))

		pam_env_size := C.int(C.strnlen(pam_env, C.size_t(maxEnvironmentVariableSize)))
		env = append(env, C.GoStringN(pam_env, pam_env_size))
	}

	return env
}

// free will end the PAM transaction (which itself will free memory) and
// then manually free any other memory allocated.
func (p *PAM) free() {
	// Only free memory one time to prevent double free bugs.
	p.once.Do(func() {
		// Terminate the PAM transaction.
		retval := C._pam_end(pamHandle, p.pamh, p.retval)
		if retval != C.PAM_SUCCESS {
			logger.WarnContext(context.Background(), "Failed to end PAM transaction", "error", p.codeToError(retval))
		}

		// Release the memory allocated for the conversation function.
		C.free(unsafe.Pointer(p.conv))

		// Release strings that were allocated when opening the PAM context.
		C.free(unsafe.Pointer(p.service_name))
		C.free(unsafe.Pointer(p.login))
	})
}

// writeStream will write to the output stream (stdout or stderr or
// equivalent).
func (p *PAM) writeStream(stream int, s string) (int, error) {
	writer := p.stdout
	if stream == syscall.Stderr {
		writer = p.stderr
	}

	// Replace \n with \r\n so the message correctly aligned.
	r := strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")
	n, err := writer.Write([]byte(r.Replace(s)))
	if err != nil {
		return n, err
	}

	return n, nil
}

// readStream will read from the input stream (stdin or equivalent).
// TODO(russjones): At some point in the future if this becomes an issue, we
// should consider supporting echo = false.
func (p *PAM) readStream(echo bool) (string, error) {
	// Limit the reader in case stdin is from /dev/zero or other infinite
	// source.
	reader := bufio.NewReader(io.LimitReader(p.stdin, int64(C.PAM_MAX_RESP_SIZE)-1))
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", trace.Wrap(err)
	}

	return text, nil
}

// codeToError returns a human readable string from the PAM error.
func (p *PAM) codeToError(returnValue C.int) error {
	// If an error is being returned, free any memory that was allocated.
	defer p.free()

	// Error strings are not allocated on the heap, so memory does not need
	// released.
	err := C._pam_strerror(pamHandle, p.pamh, returnValue)
	if err != nil {
		return trace.BadParameter("%s", C.GoString(err))
	}

	return nil
}

// BuildHasPAM returns true if the binary was build with support for PAM
// compiled in.
func BuildHasPAM() bool {
	return buildHasPAM
}

// SystemHasPAM returns true if the PAM library exists on the system.
func SystemHasPAM() bool {
	return systemHasPAM
}
