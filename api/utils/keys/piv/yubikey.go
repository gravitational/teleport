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
	"math/big"
	"strings"
	"sync"
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
)

func (y *YubiKey) sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, keyInfo hardwarekey.ContextualKeyInfo, prompt hardwarekey.Prompt, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// When using [piv.PINPolicyOnce], PIN is only required when it isn't cached in the PCSC
	// transaction internally. The piv-go prompt logic attempts to check this requirement
	// before prompting, which is generally workable. However, the PIN prompt logic is not
	// flexible enough for the retry and PIN caching mechanisms supported in Teleport. As a
	// result, we must first try signature without PIN and only prompt for PIN when we get a
	// "security status not satisfied" error ([pivGenericAuthErrCodeString]).
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

	var promptTouch promptTouch
	if ref.Policy.TouchRequired {
		promptTouch = func(ctx context.Context) error {
			return y.promptTouch(ctx, prompt, keyInfo)
		}
	}

	signature, err := y.conn.sign(ctx, ref, auth, promptTouch, rand, digest, opts)
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
		return y.conn.sign(ctx, ref, auth, promptTouch, rand, digest, opts)
	default:
		return nil, trace.Wrap(err)
	}
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

type cryptoPublicKey interface {
	Equal(x crypto.PublicKey) bool
}

// getPublicKey gets a public key from the given PIV slot.
func (y *YubiKey) getPublicKey(slot piv.Slot) (cryptoPublicKey, error) {
	slotCert, err := y.conn.attest(slot)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get slot cert on PIV slot 0x%x", slot.Key)
	}

	slotPub, ok := slotCert.PublicKey.(cryptoPublicKey)
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

func (y *YubiKey) promptTouch(ctx context.Context, prompt hardwarekey.Prompt, keyInfo hardwarekey.ContextualKeyInfo) error {
	y.promptMu.Lock()
	defer y.promptMu.Unlock()

	return prompt.Touch(ctx, keyInfo)
}

func (y *YubiKey) setPINAndPUKFromDefault(ctx context.Context, prompt hardwarekey.Prompt, keyInfo hardwarekey.ContextualKeyInfo, pinCacheTTL time.Duration) (string, error) {
	y.pinCache.mu.Lock()
	defer y.pinCache.mu.Unlock()

	// Use a longer timeout than pinPromptTimeout since this specific prompt requires the user to
	// re-type both PIN and PUK. The user might also want to save the values somewhere.
	// pinPromptTimeout just doesn't give enough time for that.
	const newPinPromptTimeout = 3 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, newPinPromptTimeout)
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

	// conn is the shared PIV connection.
	conn              *piv.YubiKey
	mu                sync.Mutex
	activeConnections int

	// exclusiveOperationMu is used to ensure that PIV operations that don't
	// support concurrency are not run concurrently.
	exclusiveOperationMu sync.RWMutex
}

// connect establishes a connection to a YubiKey PIV module and returns a release function.
// The release function should be called to properly close the shared connection.
// The connection is not immediately terminated, allowing other callers to
// use it before it's released.
// The YubiKey PIV module itself takes some additional time to handle closed
// connections, so we use a retry loop to give the PIV module time to close prior connections.
func (c *sharedPIVConnection) connect() (func(), error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	release := func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.activeConnections--
		if c.activeConnections == 0 {
			c.conn.Close()
			c.conn = nil
		}
	}

	if c.conn != nil {
		c.activeConnections++
		return release, nil
	}

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
		return nil, trace.Wrap(err)
	}

	// Backoff and retry for up to 1 second.
	retryCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	isRetryError := func(err error) bool {
		const retryError = "connecting to smart card: the smart card cannot be accessed because of other connections outstanding"
		return strings.Contains(err.Error(), retryError)
	}

	err = linearRetry.For(retryCtx, func() error {
		c.conn, err = piv.Open(c.card)
		if err != nil && !isRetryError(err) {
			return retryutils.PermanentRetryError(err)
		}
		return trace.Wrap(err)
	})
	if trace.IsLimitExceeded(err) {
		// Using PIV synchronously causes issues since only one connection is allowed at a time.
		// This shouldn't be an issue for `tsh` which primarily runs consecutively, but Teleport
		// Connect works through callbacks, etc. and may try to open multiple connections at a time.
		// If this error is being emitted more than rarely, the 1 second timeout may need to be increased.
		//
		// It's also possible that the user is running another PIV program, which may hold the PIV
		// connection indefinitely (yubikey-agent). In this case, user action is necessary, so we
		// alert them with this issue.
		return nil, trace.LimitExceeded("could not connect to YubiKey as another application is using it. Please try again once the program that uses the YubiKey, such as yubikey-agent is closed")
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	c.activeConnections++
	return release, nil
}

type promptTouch func(ctx context.Context) error

func (c *sharedPIVConnection) sign(ctx context.Context, ref *hardwarekey.PrivateKeyRef, auth piv.KeyAuth, promptTouch promptTouch, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	pivSlot, err := parsePIVSlot(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.RLock()
	defer c.exclusiveOperationMu.RUnlock()

	// Prepare the key and perform the signature with the same connection.
	// Closing the connection in between breaks the underlying PIV handle.
	priv, err := c.conn.PrivateKey(pivSlot, ref.PublicKey, auth)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, ok := priv.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("private key type %T does not implement crypto.Signer", priv)
	}

	return abandonableSign(ctx, signer, promptTouch, rand, digest, opts)
}

// abandonableSign extends [sharedPIVConnection.sign] to handle context, allowing the
// caller to stop waiting for the result if the provided context is canceled.
//
// This is necessary for hardware key signatures which sometimes require touch from the
// user to complete, which can block program termination.
//
// Important: this function only abandons the signer.Sign result, doesn't cancel it.
func abandonableSign(ctx context.Context, signer crypto.Signer, promptTouch promptTouch, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Since this function isn't fully synchronous, the goroutines below may outlive
	// the function call, especially sign which cannot be stopped once started. We
	// use buffered channels to ensure these goroutines can send even with no receiver
	// to avoid leaking.
	signatureC := make(chan []byte, 1)
	errC := make(chan error, 2)

	go func() {
		signature, err := signer.Sign(rand, digest, opts)
		if err != nil {
			errC <- err
			return
		}
		signatureC <- signature
	}()

	if promptTouch != nil {
		go func() {
			// There is no built in mechanism to prompt for touch on demand, so we simply prompt for touch after
			// a short duration in hopes of lining up with the actual YubiKey touch prompt (flashing key). In the
			// case where touch is cached, the delay prevents the prompt from firing when it isn't needed.
			select {
			case <-time.After(signTouchPromptDelay):
				if err := promptTouch(ctx); err != nil {
					errC <- promptTouch(ctx)
				}
			case <-ctx.Done():
				// prompt cached or signature canceled, skip prompt.
			}
		}()
	}

	select {
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
	case err := <-errC:
		return nil, trace.Wrap(err)
	case signature := <-signatureC:
		return signature, nil
	}
}

func (c *sharedPIVConnection) getSerialNumber() (uint32, error) {
	release, err := c.connect()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.RLock()
	defer c.exclusiveOperationMu.RUnlock()

	serial, err := c.conn.Serial()
	return serial, trace.Wrap(err)
}

func (c *sharedPIVConnection) getVersion() (piv.Version, error) {
	release, err := c.connect()
	if err != nil {
		return piv.Version{}, trace.Wrap(err)
	}
	defer release()

	// Version only requires an open connection, so we don't need to lock on [c.exclusiveOperationMu].
	return c.conn.Version(), nil
}

func (c *sharedPIVConnection) reset() error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.Lock()
	defer c.exclusiveOperationMu.Unlock()

	return trace.Wrap(c.conn.Reset())
}

func (c *sharedPIVConnection) setCertificate(key [24]byte, slot piv.Slot, cert *x509.Certificate) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.Lock()
	defer c.exclusiveOperationMu.Unlock()

	return trace.Wrap(c.conn.SetCertificate(key, slot, cert))
}

func (c *sharedPIVConnection) certificate(slot piv.Slot) (*x509.Certificate, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.Lock()
	defer c.exclusiveOperationMu.Unlock()

	cert, err := c.conn.Certificate(slot)
	return cert, trace.Wrap(err)
}

func (c *sharedPIVConnection) generateKey(key [24]byte, slot piv.Slot, opts piv.Key) (crypto.PublicKey, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.Lock()
	defer c.exclusiveOperationMu.Unlock()

	pubKey, err := c.conn.GenerateKey(key, slot, opts)
	return pubKey, trace.Wrap(err)
}

func (c *sharedPIVConnection) attest(slot piv.Slot) (*x509.Certificate, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.Lock()
	defer c.exclusiveOperationMu.Unlock()

	cert, err := c.conn.Attest(slot)
	return cert, trace.Wrap(err)
}

func (c *sharedPIVConnection) attestationCertificate() (*x509.Certificate, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.Lock()
	defer c.exclusiveOperationMu.Unlock()

	cert, err := c.conn.AttestationCertificate()
	return cert, trace.Wrap(err)
}

func (c *sharedPIVConnection) setPIN(oldPIN string, newPIN string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.RLock()
	defer c.exclusiveOperationMu.RUnlock()

	return trace.Wrap(c.conn.SetPIN(oldPIN, newPIN))
}

func (c *sharedPIVConnection) setPUK(oldPUK string, newPUK string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.RLock()
	defer c.exclusiveOperationMu.RUnlock()

	return trace.Wrap(c.conn.SetPUK(oldPUK, newPUK))
}

func (c *sharedPIVConnection) unblock(puk string, newPIN string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.RLock()
	defer c.exclusiveOperationMu.RUnlock()

	return trace.Wrap(c.conn.Unblock(puk, newPIN))
}

func (c *sharedPIVConnection) verifyPIN(pin string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	c.exclusiveOperationMu.RLock()
	defer c.exclusiveOperationMu.RUnlock()

	return trace.Wrap(c.conn.VerifyPIN(pin))
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
