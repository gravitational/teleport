//go:build touchid
// +build touchid

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

package touchid

// #cgo CFLAGS: -Wall -xobjective-c -fblocks -fobjc-arc -mmacosx-version-min=11.0
// #cgo LDFLAGS: -framework CoreFoundation -framework Foundation -framework LocalAuthentication -framework Security
// #include <stdlib.h>
// #include "authenticate.h"
// #include "context.h"
// #include "credential_info.h"
// #include "credentials.h"
// #include "diag.h"
// #include "register.h"
import "C"

import (
	"context"
	"encoding/base64"
	"fmt"
	"runtime/cgo"
	"strings"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	// rpIDUserMarker is the marker for labels containing RPID and username.
	// The marker is useful to tell apart labels written by tsh from other entries
	// (for example, a mysterious "iMessage Signing Key" shows up in some macs).
	rpIDUserMarker = "t01/"

	// rpID are domain names, so it's safe to assume they won't have spaces in them.
	// https://www.w3.org/TR/webauthn-2/#relying-party-identifier
	labelSeparator = " "

	// promptReason is the LAContext / Touch ID prompt.
	// The final prompt is: "$binary is trying to authenticate user".
	promptReason = "authenticate user"
)

const (
	kLAErrorDomain       = "com.apple.LocalAuthentication"
	kLAErrorSystemCancel = -4
)

type parsedLabel struct {
	rpID, user string
}

func makeLabel(rpID, user string) string {
	return rpIDUserMarker + rpID + labelSeparator + user
}

func parseLabel(label string) (*parsedLabel, error) {
	if !strings.HasPrefix(label, rpIDUserMarker) {
		return nil, trace.BadParameter("label has unexpected prefix: %q", label)
	}
	l := label[len(rpIDUserMarker):]

	idx := strings.Index(l, labelSeparator)
	if idx == -1 {
		return nil, trace.BadParameter("label separator not found: %q", label)
	}

	return &parsedLabel{
		rpID: l[0:idx],
		user: l[idx+1:],
	}, nil
}

var native nativeTID = &touchIDImpl{}

type touchIDImpl struct{}

func (touchIDImpl) Diag() (*DiagResult, error) {
	var resC C.DiagResult
	C.RunDiag(&resC)
	defer func() {
		C.free(unsafe.Pointer(resC.la_error_domain))
		C.free(unsafe.Pointer(resC.la_error_description))
	}()

	signed := (bool)(resC.has_signature)
	entitled := (bool)(resC.has_entitlements)
	passedLA := (bool)(resC.passed_la_policy_test)
	passedEnclave := (bool)(resC.passed_secure_enclave_test)
	laErrorCode := int64(resC.la_error_code)
	laErrorDomain := C.GoString(resC.la_error_domain)
	laErrorDescription := C.GoString(resC.la_error_description)
	if !passedLA && laErrorDescription != "" {
		logger.DebugContext(context.Background(), "Received non-empty LAError description", "description", laErrorDescription)
	}

	isAvailable := signed && entitled && passedLA && passedEnclave
	isClamshellFailure := !isAvailable &&
		// LAContext test failed.
		!passedLA &&
		// Everything else worked.
		signed &&
		entitled &&
		passedEnclave &&
		// We got an LAErrorSystemCancel error.
		laErrorDomain == kLAErrorDomain &&
		laErrorCode == kLAErrorSystemCancel

	return &DiagResult{
		HasCompileSupport:       true,
		HasSignature:            signed,
		HasEntitlements:         entitled,
		PassedLAPolicyTest:      passedLA,
		PassedSecureEnclaveTest: passedEnclave,
		IsAvailable:             isAvailable,
		isClamshellFailure:      isClamshellFailure,
	}, nil
}

//export runGoFuncHandle
func runGoFuncHandle(handle C.uintptr_t) {
	val := cgo.Handle(handle).Value()
	fn, ok := val.(func())
	if !ok {
		logger.WarnContext(context.Background(), "received unexpected function handle", "handle", logutils.TypeAttr(val))
		return
	}
	fn()
}

// touchIDContext wraps C.AuthContext into an authContext shell.
type touchIDContext struct {
	ctx *C.AuthContext
}

func (c *touchIDContext) Guard(fn func()) error {
	reasonC := C.CString(promptReason)
	defer C.free(unsafe.Pointer(reasonC))

	// Passing Go function pointers directly to CGO is not doable, so we pass a
	// handle and have an exported Go function run it.
	// See https://github.com/golang/go/wiki/cgo#function-variables.
	handle := cgo.NewHandle(fn)
	defer handle.Delete()

	var errMsgC *C.char
	defer func() { C.free(unsafe.Pointer(errMsgC)) }()

	res := C.AuthContextGuard(c.ctx, reasonC, C.uintptr_t(handle), &errMsgC)
	if res != 0 {
		errMsg := C.GoString(errMsgC)
		return errorFromStatus("guard", int(res), errMsg)
	}

	return nil
}

func (c *touchIDContext) Close() {
	if c.ctx == nil {
		return
	}
	C.AuthContextClose(c.ctx)
	c.ctx = nil
}

// getNativeContext returns the C.AuthContext within ctx, or nil.
func getNativeContext(ctx AuthContext) *C.AuthContext {
	if tctx, ok := ctx.(*touchIDContext); ok {
		return tctx.ctx
	}
	return nil
}

func (touchIDImpl) NewAuthContext() AuthContext {
	return &touchIDContext{
		ctx: &C.AuthContext{},
	}
}

func (touchIDImpl) Register(rpID, user string, userHandle []byte) (*CredentialInfo, error) {
	credentialID := uuid.NewString()
	userHandleB64 := base64.RawURLEncoding.EncodeToString(userHandle)

	var req C.CredentialInfo
	req.label = C.CString(makeLabel(rpID, user))
	req.app_label = C.CString(credentialID)
	req.app_tag = C.CString(userHandleB64)
	defer func() {
		C.free(unsafe.Pointer(req.label))
		C.free(unsafe.Pointer(req.app_label))
		C.free(unsafe.Pointer(req.app_tag))
	}()

	var errMsgC, pubKeyC *C.char
	defer func() {
		C.free(unsafe.Pointer(errMsgC))
		C.free(unsafe.Pointer(pubKeyC))
	}()

	if res := C.Register(req, &pubKeyC, &errMsgC); res != 0 {
		errMsg := C.GoString(errMsgC)
		return nil, errorFromStatus("register", int(res), errMsg)
	}

	pubKeyB64 := C.GoString(pubKeyC)
	pubKeyRaw, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &CredentialInfo{
		CredentialID: credentialID,
		publicKeyRaw: pubKeyRaw,
	}, nil
}

func (touchIDImpl) Authenticate(actx AuthContext, credentialID string, digest []byte) ([]byte, error) {
	authCtx := getNativeContext(actx)

	var req C.AuthenticateRequest
	req.app_label = C.CString(credentialID)
	req.digest = (*C.char)(C.CBytes(digest))
	req.digest_len = C.size_t(len(digest))
	defer func() {
		C.free(unsafe.Pointer(req.app_label))
		C.free(unsafe.Pointer(req.digest))
	}()

	var sigOutC, errMsgC *C.char
	defer func() {
		C.free(unsafe.Pointer(sigOutC))
		C.free(unsafe.Pointer(errMsgC))
	}()

	if res := C.Authenticate(authCtx, req, &sigOutC, &errMsgC); res != 0 {
		errMsg := C.GoString(errMsgC)
		return nil, errorFromStatus("authenticate", int(res), errMsg)
	}

	sigB64 := C.GoString(sigOutC)
	return base64.StdEncoding.DecodeString(sigB64)
}

func (touchIDImpl) FindCredentials(rpID, user string) ([]CredentialInfo, error) {
	var filterC C.LabelFilter
	if user == "" {
		filterC.kind = C.LABEL_PREFIX
	}
	filterC.value = C.CString(makeLabel(rpID, user))
	defer C.free(unsafe.Pointer(filterC.value))

	infos, res := readCredentialInfos(func(infosC **C.CredentialInfo) C.int {
		return C.FindCredentials(filterC, infosC)
	})
	if res < 0 {
		return nil, errorFromStatus("finding credentials", res, "" /* msg */)
	}
	return infos, nil
}

func (touchIDImpl) ListCredentials() ([]CredentialInfo, error) {
	reasonC := C.CString(promptReason)
	defer C.free(unsafe.Pointer(reasonC))

	var errMsgC *C.char
	defer func() { C.free(unsafe.Pointer(errMsgC)) }()

	infos, res := readCredentialInfos(func(infosOut **C.CredentialInfo) C.int {
		// ListCredentials lists all Keychain entries we have access to, without
		// prefix-filtering labels, for example.
		// Unexpected entries are removed via readCredentialInfos. This behavior is
		// intentional, as it lets us glimpse into otherwise inaccessible Keychain
		// contents.
		return C.ListCredentials(reasonC, infosOut, &errMsgC)
	})
	if res < 0 {
		errMsg := C.GoString(errMsgC)
		return nil, errorFromStatus("listing credentials", res, errMsg)
	}

	return infos, nil
}

func readCredentialInfos(find func(**C.CredentialInfo) C.int) ([]CredentialInfo, int) {
	var infosC *C.CredentialInfo
	defer func() { C.free(unsafe.Pointer(infosC)) }()

	ctx := context.Background()

	res := find(&infosC)
	if res < 0 {
		return nil, int(res)
	}

	start := unsafe.Pointer(infosC)
	size := unsafe.Sizeof(C.CredentialInfo{})
	infos := make([]CredentialInfo, 0, res)
	for i := 0; i < int(res); i++ {
		var label, appLabel, appTag, pubKeyB64, creationDate string
		{
			infoC := (*C.CredentialInfo)(unsafe.Add(start, uintptr(i)*size))

			// Get all data from infoC...
			label = C.GoString(infoC.label)
			appLabel = C.GoString(infoC.app_label)
			appTag = C.GoString(infoC.app_tag)
			pubKeyB64 = C.GoString(infoC.pub_key_b64)
			creationDate = C.GoString(infoC.creation_date)

			// ... then free it before proceeding.
			C.free(unsafe.Pointer(infoC.label))
			C.free(unsafe.Pointer(infoC.app_label))
			C.free(unsafe.Pointer(infoC.app_tag))
			C.free(unsafe.Pointer(infoC.pub_key_b64))
			C.free(unsafe.Pointer(infoC.creation_date))
		}

		// credential ID / UUID
		credentialID := appLabel

		// user@rpid
		parsedLabel, err := parseLabel(label)
		if err != nil {
			logger.DebugContext(ctx, "Skipping credential",
				"credential_id", credentialID,
				"error", err,
			)
			continue
		}

		// user handle
		userHandle, err := base64.RawURLEncoding.DecodeString(appTag)
		if err != nil {
			logger.DebugContext(ctx, "Skipping credential, unexpected application tag",
				"credential_id", credentialID,
				"app_tag", appTag,
			)
			continue
		}

		// ECDSA public key
		pubKeyRaw, err := base64.StdEncoding.DecodeString(pubKeyB64)
		if err != nil {
			logger.WarnContext(ctx, "Failed to decode public key for credential",
				"credential_id", credentialID,
				"error", err,
			)
			// Do not return or break out of the loop, it needs to run in order to
			// deallocate the structs within.
		}

		// iso8601Format is pretty close to, but not exactly the same as, RFC3339.
		const iso8601Format = "2006-01-02T15:04:05Z0700"
		createTime, err := time.Parse(iso8601Format, creationDate)
		if err != nil {
			logger.WarnContext(ctx, "Failed to parse creation time for credential",
				"creation_time", creationDate,
				"credential_id", credentialID,
				"error", err,
			)
		}

		infos = append(infos, CredentialInfo{
			CredentialID: credentialID,
			RPID:         parsedLabel.rpID,
			User: UserInfo{
				UserHandle: userHandle,
				Name:       parsedLabel.user,
			},
			CreateTime:   createTime,
			publicKeyRaw: pubKeyRaw,
		})
	}
	return infos, int(res)
}

// https://osstatus.com/search/results?framework=Security&search=-25300
const errSecItemNotFound = -25300

func (touchIDImpl) DeleteCredential(credentialID string) error {
	reasonC := C.CString(promptReason)
	defer C.free(unsafe.Pointer(reasonC))

	idC := C.CString(credentialID)
	defer C.free(unsafe.Pointer(idC))

	var errC *C.char
	defer func() { C.free(unsafe.Pointer(errC)) }()

	switch res := C.DeleteCredential(reasonC, idC, &errC); res {
	case 0: // aka success
		return nil
	case errSecItemNotFound:
		return ErrCredentialNotFound
	default:
		errMsg := C.GoString(errC)
		return errorFromStatus("delete credential", int(res), errMsg)
	}
}

func (touchIDImpl) DeleteNonInteractive(credentialID string) error {
	idC := C.CString(credentialID)
	defer C.free(unsafe.Pointer(idC))

	switch res := C.DeleteNonInteractive(idC); res {
	case 0: // aka success
		return nil
	case errSecItemNotFound:
		return ErrCredentialNotFound
	default:
		return errorFromStatus("non-interactive delete", int(res), "" /* msg */)
	}
}

func errorFromStatus(prefix string, status int, msg string) error {
	if msg != "" {
		return fmt.Errorf("%v: %v", prefix, msg)
	}
	return fmt.Errorf("%v: status %d", prefix, status)
}
