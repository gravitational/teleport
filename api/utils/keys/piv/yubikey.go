//go:build piv && !pivtest

// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package piv

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// YubiKey is a specific YubiKey PIV card.
// The [sharedPIVConnection] field makes its methods thread-safe.
type YubiKey struct {
	// conn is a shared YubiKey PIV connection.
	//
	// For each YubiKey, PIV connections claim an exclusive lock on the key's
	// PIV module until closed. In order to improve connection sharing for this
	// program without locking out other programs during extended program executions
	// (like "tsh proxy ssh"), this connections is opportunistically formed and
	// released after being unused for a few seconds.
	conn *sharedPIVConnection
	// serialNumber is the YubiKey's 8 digit serial number.
	serialNumber uint32
	// version is the YubiKey's version.
	version piv.Version
	// pinCache can be used to skip PIN prompts for keys that have PIN caching enabled.
	pinCache *pinCache

	// promptMu prevents prompting for PIN/touch repeatedly for concurrent signatures.
	// TODO(Joerger): Rather than preventing concurrent signatures, we can make the
	// PIN and touch prompts durable to concurrent signatures.
	promptMu sync.Mutex
}

// FindYubiKey finds a YubiKey PIV card by serial number. If the provided
// [serialNumber] is "0", the first YubiKey found will be returned.
func FindYubiKey(serialNumber uint32) (*YubiKey, error) {
	yubiKeyCards, err := findYubiKeyCards()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(yubiKeyCards) == 0 {
		if serialNumber != 0 {
			return nil, trace.ConnectionProblem(nil, "no YubiKey device connected with serial number %d", serialNumber)
		}
		return nil, trace.ConnectionProblem(nil, "no YubiKey device connected")
	}

	for _, card := range yubiKeyCards {
		y, err := newYubiKey(card)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if serialNumber == 0 || y.serialNumber == serialNumber {
			return y, nil
		}
	}

	return nil, trace.ConnectionProblem(nil, "no YubiKey device connected with serial number %d", serialNumber)
}

// pivCardTypeYubiKey is the PIV card type assigned to yubiKeys.
const pivCardTypeYubiKey = "yubikey"

// findYubiKeyCards returns a list of connected yubiKey PIV card names.
func findYubiKeyCards() ([]string, error) {
	cards, err := piv.Cards()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var yubiKeyCards []string
	for _, card := range cards {
		if strings.Contains(strings.ToLower(card), pivCardTypeYubiKey) {
			yubiKeyCards = append(yubiKeyCards, card)
		}
	}

	return yubiKeyCards, nil
}

func newYubiKey(card string) (*YubiKey, error) {
	y := &YubiKey{
		pinCache: newPINCache(),
		conn: &sharedPIVConnection{
			card: card,
		},
	}

	var err error
	if y.serialNumber, err = y.conn.getSerialNumber(); err != nil {
		return nil, trace.Wrap(err)
	}
	if y.version, err = y.conn.getVersion(); err != nil {
		return nil, trace.Wrap(err)
	}

	return y, nil
}

// YubiKeys require touch when signing with a private key that requires touch.
// Unfortunately, there is no good way to check whether touch is cached by the
// PIV module at a given time. In order to require touch only when needed, we
// prompt for touch after a short delay when we expect the request would succeed
// if touch were not required.
//
// There are some X factors which determine how long a request may take, such as the
// YubiKey model and firmware version, so the delays below may need to be adjusted to
// suit more models. The durations mentioned below were retrieved from testing with a
// YubiKey 5 nano (5.2.7) and a YubiKey NFC (5.4.3).
const (
	// piv.ECDSAPrivateKey.Sign consistently takes ~70 milliseconds. However, 200ms
	// should be imperceptible the the user and should avoid misfired prompts for
	// slower cards (if there are any).
	signTouchPromptDelay = time.Millisecond * 200
)

const (
	// For generic auth errors, such as when PIN is not provided, the smart card returns the error code 0x6982.
	// The piv-go library wraps error codes like this with a user readable message: "security status not satisfied".
	pivGenericAuthErrCodeString = "6982"

	// If a pcsc transaction is closed by the pcsc daemon, all operations will result in the SCARD_W_RESET_CARD error code,
	// which the piv-go library replaces with the following error message. This error can be handled by starting a new
	// transactions or reconnecting.
	//
	// See https://github.com/go-piv/piv-go/pull/173 for more details.
	//
	// TODO(Joerger): Once ^ is merged and released upstream, remove this adhoc retry.
	pcscResetCardErrMessage = "the smart card has been reset, so any shared state information is invalid"
)

func (y *YubiKey) signWithPINRetry(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, prompt hardwarekey.Prompt, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// When using [piv.PINPolicyOnce], PIN is only required when it isn't cached in the PCSC
	// transaction internally. The piv-go prompt logic attempts to check this requirement
	// before prompting, which is generally workable. However, the PIN prompt logic is not
	// flexible enough for the retry and PIN caching mechanisms supported in Teleport. As a
	// result, we must first try signature without PIN and only prompt for PIN when we get a
	// "security status not satisfied" error ([pcscResetCardErrMessage]).
	//
	// TODO(Joerger): Once https://github.com/go-piv/piv-go/pull/174 is merged upstream, we can
	// check if PIN is required and verify PIN before attempting the signature. This is a more
	// reliable method of checking the PIN requirement than the somewhat general auth error
	// returned by the failed signature.
	// IMPORTANT: Maintain the signature retry flow for firmware version 5.3.1, which has a bug
	// with checking the PIN requirement - https://github.com/gravitational/teleport/pull/36427.
	auth := piv.KeyAuth{
		PINPolicy: piv.PINPolicyNever,
	}

	signature, err := y.sign(ctx, ref, keyInfo, prompt, auth, rand, digest, opts)
	switch {
	case err == nil:
		return signature, nil
	case strings.Contains(err.Error(), pivGenericAuthErrCodeString) && ref.Policy.PINRequired:
		pin, err := y.promptPIN(ctx, prompt, hardwarekey.PINRequired, keyInfo, ref.PINCacheTTL)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Setting the [piv.PINPolicyAlways] ensures that the PIN is used and skips
		// the required check usually used with [piv.PINPolicyOnce].
		auth.PINPolicy = piv.PINPolicyAlways
		auth.PIN = pin
		return y.sign(ctx, ref, keyInfo, prompt, auth, rand, digest, opts)
	default:
		return nil, trace.Wrap(err)
	}
}

func (y *YubiKey) sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, prompt hardwarekey.Prompt, auth piv.KeyAuth, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if ref.Policy.TouchRequired {
		ctx = y.promptTouch(ctx, prompt, keyInfo)
	}

	return y.conn.sign(ctx, ref, auth, rand, digest, opts)
}

// promptTouch starts the touch prompt. The context returned is tied to the touch
// prompt so that if the user cancels the touch prompt it cancels the sign request.
func (y *YubiKey) promptTouch(ctx context.Context, prompt hardwarekey.Prompt, keyInfo hardwarekey.ContextualKeyInfo) context.Context {
	ctx, cancel := context.WithCancelCause(ctx)

	// There is no built in mechanism to prompt for touch on demand, so we simply prompt for touch after
	// a short duration in hopes of lining up with the actual YubiKey touch prompt (flashing key). In the
	// case where touch is cached, the delay prevents the prompt from firing when it isn't needed.
	go func() {
		defer cancel(nil)

		// Wait for any concurrent prompts to complete. If there is a concurrent touch prompt,
		// or touch is otherwise provided in the meantime, we can skip the prompt below.
		y.promptMu.Lock()
		defer y.promptMu.Unlock()

		select {
		case <-time.After(signTouchPromptDelay):
			err := prompt.Touch(ctx, keyInfo)
			if err != nil {
				// Cancel the entire function when an error occurs.
				// This is typically used for aborting the prompt.
				cancel(trace.Wrap(err))
			}
			return
		case <-ctx.Done():
			// touch cached, skip prompt.
			return
		}
	}()

	return ctx
}

// Reset resets the YubiKey PIV module to default settings.
func (y *YubiKey) Reset() error {
	err := y.conn.reset()
	return trace.Wrap(err)
}

// generatePrivateKey generates a new private key in the given PIV slot.
func (y *YubiKey) generatePrivateKey(slot piv.Slot, policy hardwarekey.PromptPolicy, algorithm hardwarekey.SignatureAlgorithm, pinCacheTTL time.Duration) (*hardwarekey.PrivateKeyRef, error) {
	touchPolicy := piv.TouchPolicyNever
	if policy.TouchRequired {
		touchPolicy = piv.TouchPolicyCached
	}

	pinPolicy := piv.PINPolicyNever
	if policy.PINRequired {
		pinPolicy = piv.PINPolicyOnce
	}

	var alg piv.Algorithm
	switch algorithm {
	// Use ECDSA key by default.
	case hardwarekey.SignatureAlgorithmEC256, 0:
		alg = piv.AlgorithmEC256
	case hardwarekey.SignatureAlgorithmEd25519:
		// TODO(Joerger): Currently algorithms are only specified in tests, but some users pre-generate
		// their keys in custom slots with custom algorithms, so we should try to support Ed25519 keys.
		// Currently the Ed25519 algorithm is only supported by SoloKeys and YubiKeys v5.7.x+
		return nil, trace.BadParameter("Ed25519 keys are not currently supported")
	case hardwarekey.SignatureAlgorithmRSA2048:
		alg = piv.AlgorithmRSA2048
	default:
		return nil, trace.BadParameter("unknown algorithm option %v", algorithm)
	}

	opts := piv.Key{
		Algorithm:   alg,
		PINPolicy:   pinPolicy,
		TouchPolicy: touchPolicy,
	}

	if err := y.GenerateKey(slot, opts); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := y.SetMetadataCertificate(slot, pkix.Name{
		Organization:       []string{certOrgName},
		OrganizationalUnit: []string{api.Version},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return y.getKeyRef(slot, pinCacheTTL)
}

// GenerateKey generates a new private key in the given PIV slot.
func (y *YubiKey) GenerateKey(slot piv.Slot, opts piv.Key) error {
	_, err := y.conn.generateKey(piv.DefaultManagementKey, slot, opts)
	return trace.Wrap(err)
}

// SetMetadataCertificate creates a self signed certificate and stores it in the YubiKey's
// PIV certificate slot. This certificate is purely used as metadata to determine when a
// slot is in used by a Teleport Client and is not fit to be used in cryptographic operations.
// This cert is also useful for users to discern where the key came with tools like `ykman piv info`.
func (y *YubiKey) SetMetadataCertificate(slot piv.Slot, subject pkix.Name) error {
	cert, err := SelfSignedMetadataCertificate(subject)
	if err != nil {
		return trace.Wrap(err)
	}

	err = y.conn.setCertificate(piv.DefaultManagementKey, slot, cert)
	return trace.Wrap(err)
}

// checkCertificate checks for a certificate on the PIV slot matching a Teleport client
// metadata certificate. Expected errors include [trace.NotFoundError] and [nonTeleportCertError].
func (y *YubiKey) checkCertificate(slot piv.Slot) error {
	cert, err := y.conn.certificate(slot)
	switch {
	case errors.Is(err, piv.ErrNotFound):
		return trace.NotFound("certificate not found in PIV slot 0x%x", slot.Key)
	case err != nil:
		return trace.Wrap(err)
	case !isTeleportMetadataCertificate(cert):
		return nonTeleportCertError{
			slot: slot,
			cert: cert,
		}
	}
	return nil
}

type cryptoPublicKeyI interface {
	Equal(x crypto.PublicKey) bool
}

// getPublicKey gets a public key from the given PIV slot.
func (y *YubiKey) getPublicKey(slot piv.Slot) (cryptoPublicKeyI, error) {
	slotCert, err := y.conn.attest(slot)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get slot cert on PIV slot 0x%x", slot.Key)
	}

	slotPub, ok := slotCert.PublicKey.(cryptoPublicKeyI)
	if !ok {
		return nil, trace.BadParameter("expected crypto.PublicKey but got %T", slotCert.PublicKey)
	}

	return slotPub, nil
}

// attestKey attests the key in the given PIV slot.
// The key's public key can be found in the returned slotCert.
func (y *YubiKey) attestKey(slot piv.Slot) (slotCert *x509.Certificate, attCert *x509.Certificate, att *piv.Attestation, err error) {
	slotCert, err = y.conn.attest(slot)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	attCert, err = y.conn.attestationCertificate()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	att, err = piv.Verify(attCert, slotCert)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return slotCert, attCert, att, nil
}

func (y *YubiKey) getKeyRef(slot piv.Slot, pinCacheTTL time.Duration) (*hardwarekey.PrivateKeyRef, error) {
	slotCert, attCert, att, err := y.attestKey(slot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ref := &hardwarekey.PrivateKeyRef{
		SerialNumber: y.serialNumber,
		SlotKey:      hardwarekey.PIVSlotKey(slot.Key),
		PublicKey:    slotCert.PublicKey,
		Policy: hardwarekey.PromptPolicy{
			TouchRequired: att.TouchPolicy != piv.TouchPolicyNever,
			PINRequired:   att.PINPolicy != piv.PINPolicyNever,
		},
		AttestationStatement: &hardwarekey.AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        slotCert.Raw,
					AttestationCert: attCert.Raw,
				},
			},
		},
		PINCacheTTL: pinCacheTTL,
	}

	if err := ref.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}

	return ref, nil
}

// SetPIN sets the YubiKey PIV PIN. This doesn't require user interaction like touch, just the correct old PIN.
func (y *YubiKey) SetPIN(oldPin, newPin string) error {
	err := y.conn.setPIN(oldPin, newPin)
	return trace.Wrap(err)
}

// checkOrSetPIN prompts the user for PIN and verifies it with the YubiKey.
// If the user provides the default PIN, they will be prompted to set a
// non-default PIN and PUK before continuing.
func (y *YubiKey) checkOrSetPIN(ctx context.Context, prompt hardwarekey.Prompt, keyInfo hardwarekey.ContextualKeyInfo, pinCacheTTL time.Duration) error {
	pin, err := y.promptPIN(ctx, prompt, hardwarekey.PINOptional, keyInfo, pinCacheTTL)
	if err != nil {
		return trace.Wrap(err)
	}

	if pin == piv.DefaultPIN {
		pin, err = y.setPINAndPUKFromDefault(ctx, prompt, keyInfo, pinCacheTTL)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// PIN (or PUK) prompts time out after 1 minute to prevent an indefinite hold of
// the pin cache mutex or the exclusive PC/SC transaction.
const pinPromptTimeout = time.Minute

// promptPIN prompts for PIN. If PIN caching is enabled, it verifies and caches the PIN for future calls.
func (y *YubiKey) promptPIN(ctx context.Context, prompt hardwarekey.Prompt, requirement hardwarekey.PINPromptRequirement, keyInfo hardwarekey.ContextualKeyInfo, pinCacheTTL time.Duration) (string, error) {
	y.pinCache.mu.Lock()
	defer y.pinCache.mu.Unlock()

	pin := y.pinCache.getPIN(pinCacheTTL)
	if pin != "" {
		return pin, nil
	}

	ctx, cancel := context.WithTimeout(ctx, pinPromptTimeout)
	defer cancel()

	y.promptMu.Lock()
	defer y.promptMu.Unlock()

	pin, err := prompt.AskPIN(ctx, requirement, keyInfo)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Verify that the PIN is correct before we cache it. This also caches it internally in the PC/SC transaction.
	// TODO(Joerger): In the signature pin prompt logic, we unfortunately repeat this verification
	// due to the way the upstream piv-go library handles PIN prompts.
	if err := y.verifyPIN(pin); err != nil {
		return "", trace.Wrap(err)
	}

	y.pinCache.setPIN(pin, pinCacheTTL)
	return pin, nil
}

func (y *YubiKey) setPINAndPUKFromDefault(ctx context.Context, prompt hardwarekey.Prompt, keyInfo hardwarekey.ContextualKeyInfo, pinCacheTTL time.Duration) (string, error) {
	y.pinCache.mu.Lock()
	defer y.pinCache.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, pinPromptTimeout)
	defer cancel()

	pinAndPUK, err := prompt.ChangePIN(ctx, keyInfo)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err := pinAndPUK.Validate(); err != nil {
		return "", trace.Wrap(err)
	}

	if pinAndPUK.PUKChanged {
		if err := y.conn.setPUK(piv.DefaultPUK, pinAndPUK.PUK); err != nil {
			return "", trace.Wrap(err)
		}
	}

	// unblock caches the new PIN the same way verify does.
	if err := y.conn.unblock(pinAndPUK.PUK, pinAndPUK.PIN); err != nil {
		return "", trace.Wrap(err)
	}

	y.pinCache.setPIN(pinAndPUK.PIN, pinCacheTTL)
	return pinAndPUK.PIN, nil
}

func (y *YubiKey) verifyPIN(pin string) error {
	err := y.conn.verifyPIN(pin)
	return trace.Wrap(err)
}

type sharedPIVConnection struct {
	// card is a reader name used to find and connect to this yubiKey.
	// This value may change between OS's, or with other system changes.
	card string

	// conn is a shared PIV connection. This connection is only guaranteed to be non-nil
	// and concurrency safe within a call to [doWithSharedConn].
	conn *piv.YubiKey
	// connMu is a RW mutex that protects conn. The conn is only opened/closed while under a full
	// lock, meaning that multiple callers can utilize the connection concurrently while under a
	// read lock without the risk of the connection being closed partway through a PIV command.
	connMu sync.RWMutex
	// connHolds is the number of active holds on the connection. It should be modified under
	// connMu.RLock to ensure a new hold isn't added for a closing connection.
	connHolds atomic.Int32
	// connHealthy signals whether the connection is healthy or needs to be reconnected.
	connHealthy atomic.Bool

	// attestMu prevents signatures from occurring concurrently with an attestation
	// request, which would corrupt the resulting certificate.
	attestMu sync.RWMutex
}

// doWithSharedConn holds a shared connection to perform the given function.
func doWithSharedConn[T any](c *sharedPIVConnection, do func(*piv.YubiKey) (T, error)) (T, error) {
	nilT := *new(T)

	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if err := c.holdConn(); err != nil {
		return nilT, trace.Wrap(err)
	}
	defer c.releaseConn()

	t, err := do(c.conn)

	// Usually this error occurs on Windows, which times out exclusive transactions after 5 seconds without any activity,
	// giving users only 5 seconds to answer PIN prompts. The PIN should now be cached locally, so we simply retry.
	if err != nil && strings.Contains(err.Error(), pcscResetCardErrMessage) {
		slog.DebugContext(context.Background(), "smart card connection timed out, reconnecting", "error", err)
		if err := c.reconnect(); err != nil {
			return nilT, trace.Wrap(err)
		}

		t, err = do(c.conn)
	}

	return t, trace.Wrap(err)
}

// holdConn holds an existing shared connection, or opens and holds a new shared connection.
// Unless holdConn returns an error, it must be followed by a call to releaseConn to ensure
// the connection is closed once there are no remaining holds.
//
// Must be called under [sharedPIVConnection.connMu.RLock].
func (c *sharedPIVConnection) holdConn() error {
	if c.conn == nil || !c.connHealthy.Load() {
		if err := c.connect(); err != nil {
			return trace.Wrap(err)
		}
	}

	c.connHolds.Add(1)
	return nil
}

// releaseConn releases a hold on a shared connection and,
// if there are no remaining holds, closes the connection.
//
// Must be called under [sharedPIVConnection.connMu.RLock].
func (c *sharedPIVConnection) releaseConn() {
	remaining := c.connHolds.Add(-1)

	// If there are no remaining holds on the connection, close it.
	if remaining == 0 {
		c.connMu.RUnlock()
		c.connMu.Lock()

		// Double check that a new hold wasn't added while waiting for the full lock.
		if c.connHolds.Load() == 0 {
			c.conn.Close()
			c.conn = nil
		}

		c.connMu.Unlock()
		c.connMu.RLock()
	}
}

// reconnect marks the connection as unhealthy and waits for a new connection.
// A new connection will not be created until all consumers of the unhealthy
// connection complete. reconnect supports multiple concurrent callers.
//
// Must be called under [sharedPIVConnection.connMu.RLock].
func (c *sharedPIVConnection) reconnect() error {
	// Prevent new callers from holding the unhealthy connection.
	c.connHealthy.Store(false)
	return c.connect()
}

// connect establishes a connection to a YubiKey PIV module and returns a release function.
// The release function should be called to properly close the shared connection.
// The connection is not immediately terminated, allowing other callers to
// use it before it's released.
// The YubiKey PIV module itself takes some additional time to handle closed
// connections, so we use a retry loop to give the PIV module time to close prior connections.
//
// Must be called under [sharedPIVConnection.connMu.RLock].
func (c *sharedPIVConnection) connect() error {
	c.connMu.RUnlock()
	c.connMu.Lock()
	defer c.connMu.RLock()
	defer c.connMu.Unlock()

	// Check if there is an existing, healthy connection.
	if c.conn != nil && c.connHealthy.Load() {
		return nil
	}

	ctx := context.Background()
	linearRetry, err := retryutils.NewLinear(retryutils.LinearConfig{
		// If a PIV connection has just been closed, it take ~5 ms to become
		// available to new connections. For this reason, we initially wait a
		// short 10ms before stepping up to a longer 50ms retry.
		First: time.Millisecond * 10,
		Step:  time.Millisecond * 10,
		// Since PIV modules only allow a single connection, it is a bottleneck
		// resource. To maximize usage, we use a short 50ms retry to catch the
		// connection opening up as soon as possible.
		Max: time.Millisecond * 50,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	isRetryError := func(err error) bool {
		const retryError = "connecting to smart card: the smart card cannot be accessed because of other connections outstanding"
		return strings.Contains(err.Error(), retryError)
	}

	tryConnect := func() error {
		c.conn, err = piv.Open(c.card)
		if err != nil && !isRetryError(err) {
			return retryutils.PermanentRetryError(err)
		}
		return trace.Wrap(err)
	}

	// Backoff and retry for up to 1 second.
	retryCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err = linearRetry.For(retryCtx, tryConnect); err != nil {
		// Using PIV synchronously causes issues since only one connection is allowed at a time.
		// This shouldn't be an issue for `tsh` which primarily runs consecutively, but Teleport
		// Connect works through callbacks, etc. and may try to open multiple connections at a time.
		// If this error is being emitted more than rarely, the 1 second timeout may need to be increased.
		//
		// It's also possible that the user is running another PIV program, which may hold the PIV
		// connection indefinitely (yubikey-agent). In this case, user action is necessary, so we
		// alert them with this issue.
		if trace.IsLimitExceeded(err) {
			slog.WarnContext(ctx, "failed to connect to YubiKey as it is currently in use by another process. "+
				"This can occur when running multiple Teleport clients simultaneously, or running long lived PIV "+
				"applications like yubikey-agent. Try again once other PIV processes have completed.")
			return trace.LimitExceeded("failed to connect to YubiKey as it is currently in use by another process")
		}

		return trace.Wrap(err)
	}

	c.connHealthy.Store(true)
	return nil
}

func (c *sharedPIVConnection) sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, auth piv.KeyAuth, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	pivSlot, err := parsePIVSlot(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.attestMu.RLock()
	defer c.attestMu.RUnlock()

	return doWithSharedConn(c, func(yk *piv.YubiKey) ([]byte, error) {
		// Prepare the key and perform the signature with the same connection.
		// Closing the connection in between breaks the underlying PIV handle.
		priv, err := yk.PrivateKey(pivSlot, ref.PublicKey, auth)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		signer, ok := priv.(crypto.Signer)
		if !ok {
			return nil, trace.BadParameter("private key type %T does not implement crypto.Signer", priv)
		}

		return abandonableSign(ctx, signer, rand, digest, opts)
	})
}

// abandonableSign is a wrapper around signer.Sign.
// It enhances the functionality of signer.Sign by allowing the caller to stop
// waiting for the result if the provided context is canceled.
// It is especially important for WarmupHardwareKey,
// where waiting for the user providing a PIN/touch could block program termination.
// Important: this function only abandons the signer.Sign result, doesn't cancel it.
func abandonableSign(ctx context.Context, signer crypto.Signer, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	type signResult struct {
		signature []byte
		err       error
	}

	signResultCh := make(chan signResult)
	go func() {
		if err := ctx.Err(); err != nil {
			return
		}
		signature, err := signer.Sign(rand, digest, opts)
		select {
		case <-ctx.Done():
		case signResultCh <- signResult{signature: signature, err: trace.Wrap(err)}:
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-signResultCh:
		return result.signature, trace.Wrap(result.err)
	}
}

func (c *sharedPIVConnection) getSerialNumber() (uint32, error) {
	return doWithSharedConn(c, func(yk *piv.YubiKey) (uint32, error) {
		serial, err := yk.Serial()
		return serial, trace.Wrap(err)
	})
}

func (c *sharedPIVConnection) getVersion() (piv.Version, error) {
	return doWithSharedConn(c, func(yk *piv.YubiKey) (piv.Version, error) {
		return yk.Version(), nil
	})
}

func (c *sharedPIVConnection) reset() error {
	_, err := doWithSharedConn(c, func(yk *piv.YubiKey) (any, error) {
		err := yk.Reset()
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func (c *sharedPIVConnection) setCertificate(key [24]byte, slot piv.Slot, cert *x509.Certificate) error {
	_, err := doWithSharedConn(c, func(yk *piv.YubiKey) (any, error) {
		err := yk.SetCertificate(key, slot, cert)
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func (c *sharedPIVConnection) certificate(slot piv.Slot) (*x509.Certificate, error) {
	return doWithSharedConn(c, func(yk *piv.YubiKey) (*x509.Certificate, error) {
		cert, err := yk.Certificate(slot)
		return cert, trace.Wrap(err)
	})
}

func (c *sharedPIVConnection) generateKey(key [24]byte, slot piv.Slot, opts piv.Key) (crypto.PublicKey, error) {
	return doWithSharedConn(c, func(yk *piv.YubiKey) (crypto.PublicKey, error) {
		pub, err := yk.GenerateKey(key, slot, opts)
		return pub, trace.Wrap(err)
	})
}

func (c *sharedPIVConnection) attest(slot piv.Slot) (*x509.Certificate, error) {
	c.attestMu.Lock()
	defer c.attestMu.Unlock()

	return doWithSharedConn(c, func(yk *piv.YubiKey) (*x509.Certificate, error) {
		cert, err := yk.Attest(slot)
		return cert, trace.Wrap(err)
	})
}

func (c *sharedPIVConnection) attestationCertificate() (*x509.Certificate, error) {
	return doWithSharedConn(c, func(yk *piv.YubiKey) (*x509.Certificate, error) {
		cert, err := yk.AttestationCertificate()
		return cert, trace.Wrap(err)
	})
}

func (c *sharedPIVConnection) setPIN(oldPIN string, newPIN string) error {
	_, err := doWithSharedConn(c, func(yk *piv.YubiKey) (any, error) {
		err := yk.SetPIN(oldPIN, newPIN)
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func (c *sharedPIVConnection) setPUK(oldPUK string, newPUK string) error {
	_, err := doWithSharedConn(c, func(yk *piv.YubiKey) (any, error) {
		err := yk.SetPUK(oldPUK, newPUK)
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func (c *sharedPIVConnection) unblock(puk string, newPIN string) error {
	_, err := doWithSharedConn(c, func(yk *piv.YubiKey) (any, error) {
		err := yk.Unblock(puk, newPIN)
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func (c *sharedPIVConnection) verifyPIN(pin string) error {
	_, err := doWithSharedConn(c, func(yk *piv.YubiKey) (any, error) {
		err := yk.VerifyPIN(pin)
		return nil, trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func parsePIVSlot(slotKey hardwarekey.PIVSlotKey) (piv.Slot, error) {
	switch uint32(slotKey) {
	case piv.SlotAuthentication.Key:
		return piv.SlotAuthentication, nil
	case piv.SlotSignature.Key:
		return piv.SlotSignature, nil
	case piv.SlotKeyManagement.Key:
		return piv.SlotKeyManagement, nil
	case piv.SlotCardAuthentication.Key:
		return piv.SlotCardAuthentication, nil
	default:
		return piv.Slot{}, trace.BadParameter("invalid slot %X", slotKey)
	}
}

// certOrgName is used to identify Teleport Client self-signed certificates stored in yubiKey PIV slots.
const certOrgName = "teleport"

func SelfSignedMetadataCertificate(subject pkix.Name) (*x509.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit) // see crypto/tls/generate_cert.go
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      subject,
		PublicKey:    priv.Public(),
	}

	if cert.Raw, err = x509.CreateCertificate(rand.Reader, cert, cert, priv.Public(), priv); err != nil {
		return nil, trace.Wrap(err)
	}
	return cert, nil
}

func isTeleportMetadataCertificate(cert *x509.Certificate) bool {
	return len(cert.Subject.Organization) > 0 && cert.Subject.Organization[0] == certOrgName
}

type nonTeleportCertError struct {
	slot piv.Slot
	cert *x509.Certificate
}

func (e nonTeleportCertError) Error() string {
	// Gather a small list of user-readable x509 certificate fields to display to the user.
	sum := sha256.Sum256(e.cert.Raw)
	fingerPrint := hex.EncodeToString(sum[:])
	return fmt.Sprintf(`Certificate in YubiKey PIV slot %q is not a Teleport client cert:
Slot %s:
	Algorithm:		%v
	Subject DN:		%v
	Issuer DN:		%v
	Serial:			%v
	Fingerprint:	%v
	Not before:		%v
	Not after:		%v
`,
		e.slot, e.slot,
		e.cert.SignatureAlgorithm,
		e.cert.Subject,
		e.cert.Issuer,
		e.cert.SerialNumber,
		fingerPrint,
		e.cert.NotBefore,
		e.cert.NotAfter,
	)
}

const agentRequiresTeleportCertMessage = "hardware key agent cannot perform signatures on PIV slots that aren't configured for Teleport. " +
	"The PIV slot should be configured automatically by the Teleport client during login. If you are " +
	"are configuring the PIV slot manually, you must also generate a certificate in the slot with " +
	"\"teleport\" as the organization name: " +
	"e.g. \"ykman piv keys generate -a ECCP256 9a pub.pem && ykman piv certificate generate 9a pub.pem -s O=teleport\""
