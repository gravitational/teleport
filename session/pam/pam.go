//go:build pam && cgo

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
// typedef const struct pam_message const_pam_message;
// extern int handlerCallback(int n, const_pam_message **msg, struct pam_response *resp, size_t data);
// extern struct pam_conv *make_pam_conv(size_t);
// extern int _pam_start(void *, const char *, const char *, const struct pam_conv *, pam_handle_t **);
// extern int _pam_putenv(void *, pam_handle_t *, const char *);
// extern int _pam_set_item(void *, pam_handle_t *, int, const void *);
// extern int _pam_end(void *, pam_handle_t *, int);
// extern int _pam_authenticate(void *, pam_handle_t *, int);
// extern int _pam_acct_mgmt(void *, pam_handle_t *, int);
// extern int _pam_setcred(void *, pam_handle_t *, int);
// extern int _pam_open_session(void *, pam_handle_t *, int);
// extern int _pam_close_session(void *, pam_handle_t *, int);
// extern char **_pam_getenvlist(void *handle, pam_handle_t *pamh);
// extern const char *_pam_strerror(void *, pam_handle_t *, int);
// extern int _pam_envlist_len(char **);
// extern char * _pam_getenv(char **, int);
import "C"

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/user"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"github.com/gravitational/trace"
	"golang.org/x/sys/unix"

	"github.com/gravitational/teleport/session/logconstants"
	"github.com/gravitational/teleport/session/pam/pamcfg"
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

// logComponent is the value for the component attribute in logs for this
// package; it should be paired with [logconstants.ComponentKey]. Using a
// package-level logger with this attribute created with [slog.With] would not
// synchronize correctly with [slog.SetDefault] and copying the packagelogger
// construction from lib/utils/log doesn't seem to be worth it when we have
// literally four log lines to deal with.
const logComponent = "pam"

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

	// maxResponseSize is the maximum size of a message to accept as a response to
	// a PAM message. It is one less than the actual max size so the string passed
	// to C can be null-terminated.
	maxResponseSize = C.PAM_MAX_RESP_SIZE - 1

	// maxEnvironmentVariableSize is the maximum size of any environment
	// variable. Even though pam_env.so sets this to a maximum of 1024, set
	// it to be 10x that because not all PAM modules follow that convention. [1]
	//
	// [1] https://github.com/linux-pam/linux-pam/blob/master/modules/pam_env/pam_env.c#L55
	maxEnvironmentVariableSize = 1024 * 10
)

const (
	stylePromptEchoOff = C.PAM_PROMPT_ECHO_OFF
	stylePromptEchoOn  = C.PAM_PROMPT_ECHO_ON
	styleErrorMessage  = C.PAM_ERROR_MSG
	styleTextInfo      = C.PAM_TEXT_INFO
)

// handler is used to register and find instances of *PAM at the package level
// to enable callbacks from C code.
type handler interface {
	// converse responds to a batch of PAM messages. The returned string slice is
	// the same length as the input slice; it will contain nil strings for
	// messages that don't require responses.
	converse([]pamMessage) ([]*string, error)
}

var (
	handlerMu    sync.Mutex
	handlerCount int
	handlers     map[int]handler = make(map[int]handler)
)

//export handlerCallback
func handlerCallback(n C.int, messages **C.const_pam_message, responses *C.struct_pam_response, index C.size_t) C.int {
	handler, err := lookupHandler(int(index))
	if err != nil {
		slog.ErrorContext(
			context.Background(), "unable to find PAM conversation handler",
			logconstants.ComponentKey, logComponent,
			"error", err,
		)
		return 1
	}

	if messages == nil {
		slog.ErrorContext(
			context.Background(), "received null message struct in PAM conversation, this is likely a bug in a PAM module",
		)
		return 1
	}
	pamMessages := make([]pamMessage, 0, n)
	for _, msg := range unsafe.Slice(messages, n) {
		if msg == nil || msg.msg == nil {
			slog.ErrorContext(
				context.Background(), "received null message in PAM conversation, this is likely a bug in a PAM module",
				logconstants.ComponentKey, logComponent,
			)
			return 1
		}
		pamMessages = append(pamMessages, pamMessage{
			style: int(msg.msg_style),
			msg:   C.GoStringN(msg.msg, C.int(C.strnlen(msg.msg, C.size_t(maxMessageSize)))),
		})
	}
	pamResponses, err := handler.converse(pamMessages)
	if err != nil {
		slog.ErrorContext(
			context.Background(), "unable to handle pam conversation",
			logconstants.ComponentKey, logComponent,
			"error", err,
		)
		return 1
	}
	responseSlice := unsafe.Slice(responses, n)
	for i, resp := range pamResponses {
		if resp != nil {
			// No need to worry about freeing these strings, that's the PAM conversation
			// caller's job.
			responseSlice[i].resp = C.CString(*resp)
		}
	}
	return 0
}

// noopHandler is a handler that returns empty responses.
type noopHandler struct{}

func (h noopHandler) converse(messages []pamMessage) ([]*string, error) {
	return make([]*string, len(messages)), nil
}

// registerHandler will register a instance of *PAM with the package level
// handlers to support callbacks from C.
func registerHandler(p *PAMContext) int {
	handlerMu.Lock()
	defer handlerMu.Unlock()

	// The make_pam_conv function allocates struct pam_conv on the heap. It will
	// be released by Close function.
	handlerCount = handlerCount + 1
	p.conv = C.make_pam_conv(C.size_t(handlerCount))
	handlers[handlerCount] = noopHandler{}

	return handlerCount
}

// setHandler sets the handler for the given index in the package level map.
func setHandler(handlerIndex int, h handler) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	handlers[handlerIndex] = h
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

var (
	buildHasPAM  bool = true
	systemHasPAM bool = false
)

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

// PAMContext is used to create a PAM context and initiate PAMContext transactions to check
// the users account and open/close a session.
type PAMContext struct {
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

	// cLogin is the *nix login that that is being used.
	login string

	// service_name is the name of the PAM policy to use.
	service_name *C.char

	// cLogin is the login as a C string.
	cLogin *C.char

	// handlerIndex is the index to the package level handler map.
	handlerIndex int

	// once is used to ensure that any memory allocated by C only has free
	// called on it once.
	once sync.Once
}

// Open creates a PAM context and initiates a PAM transaction.
func Open(config *pamcfg.PAMConfig) (_ *PAMContext, retErr error) {
	if config == nil {
		return nil, trace.BadParameter("PAM configuration is required.")
	}
	err := config.CheckDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p := &PAMContext{
		pamh:   nil,
		stdin:  config.Stdin,
		stdout: config.Stdout,
		stderr: config.Stderr,
		login:  config.Login,
	}

	// Both config.ServiceName and config.Username convert between Go strings to
	// C strings. Since the C strings are allocated on the heap in Go code, this
	// memory must be released (and will be on the call to the Close method).
	p.service_name = C.CString(config.ServiceName)
	p.cLogin = C.CString(config.Login)

	// C code does not know that this PAM context exists. To ensure the
	// conversation function can get messages to the right context, a handle
	// registry at the package level is created (handlers). Each instance of the
	// PAM context has it's own handle which is used to communicate between C
	// and a instance of a PAM context.
	p.handlerIndex = registerHandler(p)

	defer func() {
		if retErr != nil {
			unregisterHandler(p.handlerIndex)
			p.free()
		}
	}()

	setHandler(p.handlerIndex, &printHandler{
		stdin:  p.stdin,
		stdout: p.stdout,
		stderr: p.stderr,
	})

	// Create and initialize a PAM context. The pam_start function will
	// allocate pamh if needed and the pam_end function will release any
	// allocated memory.
	p.retval = C._pam_start(pamHandle, p.service_name, p.cLogin, p.conv, &p.pamh)
	if p.retval != C.PAM_SUCCESS {
		return nil, p.codeToError(p.retval)
	}

	// Set tty and remote host so pam_systemd can classify the session correctly.
	if config.TTYName != "" {
		pamTTY := C.CString(config.TTYName)
		defer C.free(unsafe.Pointer(pamTTY))
		p.retval = C._pam_set_item(pamHandle, p.pamh, C.PAM_TTY, unsafe.Pointer(pamTTY))
		if p.retval != C.PAM_SUCCESS {
			return nil, p.codeToError(p.retval)
		}
	}

	if config.RemoteHost != "" {
		pamRHost := C.CString(config.RemoteHost)
		defer C.free(unsafe.Pointer(pamRHost))
		p.retval = C._pam_set_item(pamHandle, p.pamh, C.PAM_RHOST, unsafe.Pointer(pamRHost))
		if p.retval != C.PAM_SUCCESS {
			return nil, p.codeToError(p.retval)
		}
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
		p.retval = C._pam_putenv(pamHandle, p.pamh, kv)
		if p.retval != C.PAM_SUCCESS {
			return nil, p.codeToError(p.retval)
		}
	}

	return p, nil
}

// Account triggers the "account" PAM hooks for this login.
//
// Check that the *nix account is valid. Checking an account varies based off
// the PAM modules used in the account stack. Typically this consists of
// checking if the account is expired or has access restrictions.
//
// Note: This function does not perform any authentication!
func (p *PAMContext) Account() error {
	p.retval = C._pam_acct_mgmt(pamHandle, p.pamh, 0)
	if p.retval != C.PAM_SUCCESS {
		return p.codeToError(p.retval)
	}
	return nil
}

// Authenticate triggers the "auth" PAM hooks for this login.
//
// These would perform any extra authentication steps configured in the PAM
// stack, like per-session 2FA.
func (p *PAMContext) Authenticate() error {
	p.retval = C._pam_authenticate(pamHandle, p.pamh, 0)
	if p.retval != C.PAM_SUCCESS {
		return p.codeToError(p.retval)
	}
	return nil
}

// Session is a PAM session, created by pam_open_session. It must be closed
// when finished.
type Session struct {
	pamCtx *PAMContext
	// targetGroups is the list of group IDs to assume when starting the session.
	// It is valid for this list to be empty, so only a nil slice indicates that
	// setgroups should not be called.
	targetGroups []int
	// originalGroups is the list of group IDs to return to when the session closes.
	originalGroups []int
}

func getUserGroups(u *user.User) ([]int, error) {
	groupIDStrings, err := u.GroupIds()
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	groupIDs := make([]int, 0, len(groupIDStrings))
	for _, gidStr := range groupIDStrings {
		gid, err := strconv.Atoi(gidStr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		groupIDs = append(groupIDs, gid)
	}
	return groupIDs, nil
}

func (p *PAMContext) newSession() (*Session, error) {
	s := &Session{
		pamCtx: p,
	}
	// setgroups is discouraged on mac, so we won't try.
	if runtime.GOOS == "darwin" {
		return s, nil
	}
	// Check if setgroups will be necessary.
	self, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	selfGroupIDs, err := os.Getgroups()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slices.Sort(selfGroupIDs)

	target, err := user.Lookup(s.pamCtx.login)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// If the session is for us, no need to change groups.
	if target.Uid == self.Uid && target.Gid == self.Gid {
		return s, nil
	}
	targetGroupIDs, err := getUserGroups(target)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	slices.Sort(targetGroupIDs)
	if slices.Equal(selfGroupIDs, targetGroupIDs) {
		return s, nil
	}
	s.originalGroups = selfGroupIDs
	s.targetGroups = targetGroupIDs
	return s, nil
}

// SetGroups updates credentials for an exec command so it will have the
// correct groups.
func (s *Session) SetGroups(cred *syscall.Credential) {
	if s.targetGroups == nil || cred.NoSetGroups {
		return
	}
	cred.Groups = make([]uint32, 0, len(s.targetGroups))
	for _, gid := range s.targetGroups {
		cred.Groups = append(cred.Groups, uint32(gid))
	}
}

// OpenSession triggers the "session" PAM hooks for this login.
//
// Open a user session. Opening a session varies based off the PAM modules
// used in the "session" stack. Opening a session typically consists of
// printing the MOTD, mounting a home directory, updating auth.log.
func (p *PAMContext) OpenSession() (*Session, error) {
	sess, err := p.newSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If pam_group.so is in use, it will expect the caller to already have the
	// user's supplementary groups. Assume them now.
	if sess.targetGroups != nil {
		if err := unix.Setgroups(sess.targetGroups); err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	p.retval = C._pam_setcred(pamHandle, p.pamh, C.PAM_ESTABLISH_CRED)
	if p.retval != C.PAM_SUCCESS {
		return nil, p.codeToError(p.retval)
	}
	if sess.targetGroups != nil {
		// If pam_group.so added groups, save them.
		newGroups, err := os.Getgroups()
		if err != nil {
			_ = C._pam_setcred(pamHandle, p.pamh, C.PAM_DELETE_CRED)
			if sess.originalGroups != nil {
				_ = unix.Setgroups(sess.originalGroups)
			}
			return nil, trace.ConvertSystemError(err)
		}
		sess.targetGroups = newGroups
	}

	p.retval = C._pam_open_session(pamHandle, p.pamh, 0)
	if p.retval != C.PAM_SUCCESS {
		_ = C._pam_setcred(pamHandle, p.pamh, C.PAM_DELETE_CRED)
		if sess.originalGroups != nil {
			_ = unix.Setgroups(sess.originalGroups)
		}
		return nil, p.codeToError(p.retval)
	}
	return sess, nil
}

// Close closes the PAM session. Closing a session can entail anything from
// unmounting a home directory and updating auth.log.
func (s *Session) Close() error {
	p := s.pamCtx
	errors := make([]error, 0, 3)
	if retval := C._pam_close_session(pamHandle, p.pamh, 0); retval != C.PAM_SUCCESS {
		p.retval = retval
		errors = append(errors, p.codeToError(p.retval))
	}
	if retval := C._pam_setcred(pamHandle, p.pamh, C.PAM_DELETE_CRED); retval != C.PAM_SUCCESS {
		if p.retval == 0 {
			p.retval = retval
		}
		errors = append(errors, p.codeToError(retval))
	}
	if s.originalGroups != nil {
		if err := unix.Setgroups(s.originalGroups); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// Close will close the PAM context and release any allocated memory.
func (p *PAMContext) Close() error {
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
func (p *PAMContext) Environment() ([]string, error) {
	// Get list of additional environment variables requested from PAM.
	pam_envlist := C._pam_getenvlist(pamHandle, p.pamh)
	if pam_envlist == nil {
		return nil, trace.BadParameter("failed to find PAM getenvlist symbol")
	}
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

	return env, nil
}

// free will end the PAM transaction (which itself will free memory) and
// then manually free any other memory allocated.
func (p *PAMContext) free() {
	// Only free memory one time to prevent double free bugs.
	p.once.Do(func() {
		// Terminate the PAM transaction.
		p.retval = C._pam_end(pamHandle, p.pamh, p.retval)
		if p.retval != C.PAM_SUCCESS {
			slog.WarnContext(context.Background(),
				"Failed to end PAM transaction",
				logconstants.ComponentKey, logComponent,
				"error", p.codeToError(p.retval),
			)
		}

		// Release the memory allocated for the conversation function.
		C.free(unsafe.Pointer(p.conv))

		// Release strings that were allocated when opening the PAM context.
		C.free(unsafe.Pointer(p.service_name))
		C.free(unsafe.Pointer(p.cLogin))
	})
}

// codeToError returns a human readable string from the PAM error.
func (p *PAMContext) codeToError(retval C.int) error {
	if retval == 0 {
		return nil
	}
	err := C._pam_strerror(pamHandle, p.pamh, retval)
	if err != nil {
		return trace.BadParameter("%s", C.GoString(err))
	}

	// Return a generic error message with the error code if pam_strerror
	// somehow failed to return a string.
	return trace.Errorf("PAM call failed with error code %d", retval)
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
