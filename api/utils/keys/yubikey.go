//go:build piv && !pivtest

/*
Copyright 2022 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keys

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-piv/piv-go/piv"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api"
	attestationv1 "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// YubiKey is a specific YubiKey PIV card.
type YubiKey struct {
	// conn is a shared YubiKey PIV connection.
	//
	// PIV connections claim an exclusive lock on the PIV module until closed.
	// In order to improve connection sharing for this program without locking
	// out other programs during extended program executions (like "tsh proxy ssh"),
	// this connections is opportunistically formed and released after being
	// unused for a few seconds.
	*sharedPIVConnection
	// serialNumber is the YubiKey's 8 digit serial number.
	serialNumber uint32
	// version is the YubiKey's version.
	version piv.Version
}

func newYubiKey(card string) (*YubiKey, error) {
	y := &YubiKey{
		sharedPIVConnection: &sharedPIVConnection{
			card: card,
		},
	}

	var err error
	if y.serialNumber, err = y.getSerialNumber(); err != nil {
		return nil, trace.Wrap(err)
	}
	if y.version, err = y.getVersion(); err != nil {
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

func (y *YubiKey) sign(ctx context.Context, ref *HardwarePrivateKeyRef, prompt HardwareKeyPrompt, rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	// Lock the connection for the entire duration of the sign
	// process. Without this, the connection will be released,
	// leading to a failure when providing PIN or touch input:
	// "verify pin: transmitting request: the supplied handle was invalid".
	release, err := y.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()

	var touchPromptDelayTimer *time.Timer
	if ref.PrivateKeyPolicy.IsHardwareKeyTouchVerified() {
		touchPromptDelayTimer = time.NewTimer(signTouchPromptDelay)
		defer touchPromptDelayTimer.Stop()

		go func() {
			select {
			case <-touchPromptDelayTimer.C:
				// Prompt for touch after a delay, in case the function succeeds without touch due to a cached touch.
				err := prompt.Touch(ctx, KeyInfo{})
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
	}

	promptPIN := func() (string, error) {
		// touch prompt delay is disrupted by pin prompts. To prevent misfired
		// touch prompts, pause the timer for the duration of the pin prompt.
		if touchPromptDelayTimer != nil {
			if touchPromptDelayTimer.Stop() {
				defer touchPromptDelayTimer.Reset(signTouchPromptDelay)
			}
		}
		pass, err := prompt.AskPIN(ctx, PINRequired, KeyInfo{})
		return pass, trace.Wrap(err)
	}

	pinPolicy := piv.PINPolicyNever
	if ref.PrivateKeyPolicy.IsHardwareKeyPINVerified() {
		pinPolicy = piv.PINPolicyOnce
	}

	auth := piv.KeyAuth{
		PINPrompt: promptPIN,
		PINPolicy: pinPolicy,
	}

	// YubiKeys with firmware version 5.3.1 have a bug where insVerify(0x20, 0x00, 0x80, nil)
	// clears the PIN cache instead of performing a non-mutable check. This causes the signature
	// with pin policy "once" to fail unless PIN is provided for each call. We can avoid this bug
	// by skipping the insVerify check and instead manually retrying with a PIN prompt only when
	// the signature fails.
	manualRetryWithPIN := false
	fw531 := piv.Version{Major: 5, Minor: 3, Patch: 1}
	if auth.PINPolicy == piv.PINPolicyOnce && y.conn.Version() == fw531 {
		// Set the keys PIN policy to never to skip the insVerify check. If PIN was provided in
		// a previous recent call, the signature will succeed as expected of the "once" policy.
		auth.PINPolicy = piv.PINPolicyNever
		manualRetryWithPIN = true
	}

	pivSlot, err := parsePIVSlot(ref.SlotKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := y.privateKey(pivSlot, ref.PublicKey, auth)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, trace.BadParameter("private key type %T does not implement crypto.Signer", privateKey)
	}

	// For generic auth errors, such as when PIN is not provided, the smart card returns the error code 0x6982.
	// The piv-go library wraps error codes like this with a user readable message: "security status not satisfied".
	const pivGenericAuthErrCodeString = "6982"

	signature, err := abandonableSign(ctx, signer, rand, digest, opts)
	switch {
	case err == nil:
		return signature, nil
	case manualRetryWithPIN && strings.Contains(err.Error(), pivGenericAuthErrCodeString):
		pin, err := promptPIN()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := y.verifyPIN(pin); err != nil {
			return nil, trace.Wrap(err)
		}
		signature, err := abandonableSign(ctx, signer, rand, digest, opts)
		return signature, trace.Wrap(err)
	default:
		return nil, trace.Wrap(err)
	}
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

// Reset resets the YubiKey PIV module to default settings.
func (y *YubiKey) Reset() error {
	err := y.reset()
	return trace.Wrap(err)
}

// generatePrivateKey generates a new private key in the given PIV slot.
func (y *YubiKey) generatePrivateKey(slot piv.Slot, requiredKeyPolicy PrivateKeyPolicy) (*HardwarePrivateKeyRef, error) {
	touchPolicy := piv.TouchPolicyNever
	if requiredKeyPolicy.IsHardwareKeyTouchVerified() {
		touchPolicy = piv.TouchPolicyCached
	}

	pinPolicy := piv.PINPolicyNever
	if requiredKeyPolicy.IsHardwareKeyPINVerified() {
		pinPolicy = piv.PINPolicyOnce
	}

	opts := piv.Key{
		Algorithm:   piv.AlgorithmEC256,
		PINPolicy:   pinPolicy,
		TouchPolicy: touchPolicy,
	}

	pub, err := y.generateKey(piv.DefaultManagementKey, slot, opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	slotCert, err := y.attest(slot)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	attCert, err := y.attestationCertificate()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := y.SetMetadataCertificate(slot, pkix.Name{
		Organization:       []string{certOrgName},
		OrganizationalUnit: []string{api.Version},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &HardwarePrivateKeyRef{
		SerialNumber:     y.serialNumber,
		SlotKey:          slot.Key,
		PublicKey:        pub,
		PrivateKeyPolicy: requiredKeyPolicy,
		AttestationStatement: &AttestationStatement{
			AttestationStatement: &attestationv1.AttestationStatement_YubikeyAttestationStatement{
				YubikeyAttestationStatement: &attestationv1.YubiKeyAttestationStatement{
					SlotCert:        slotCert.Raw,
					AttestationCert: attCert.Raw,
				},
			},
		},
	}, nil
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

	err = y.setCertificate(piv.DefaultManagementKey, slot, cert)
	return trace.Wrap(err)
}

// getCertificate gets a certificate from the given PIV slot.
func (y *YubiKey) getCertificate(slot piv.Slot) (*x509.Certificate, error) {
	cert, err := y.certificate(slot)
	return cert, trace.Wrap(err)
}

// attestKey attests the key in the given PIV slot.
// The key's public key can be found in the returned slotCert.
func (y *YubiKey) attestKey(slot piv.Slot) (slotCert *x509.Certificate, attCert *x509.Certificate, att *piv.Attestation, err error) {
	slotCert, err = y.attest(slot)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	attCert, err = y.attestationCertificate()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	att, err = piv.Verify(attCert, slotCert)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return slotCert, attCert, att, nil
}

// SetPIN sets the YubiKey PIV PIN. This doesn't require user interaction like touch, just the correct old PIN.
func (y *YubiKey) SetPIN(oldPin, newPin string) error {
	err := y.setPIN(oldPin, newPin)
	return trace.Wrap(err)
}

// checkOrSetPIN prompts the user for PIN and verifies it with the YubiKey.
// If the user provides the default PIN, they will be prompted to set a
// non-default PIN and PUK before continuing.
func (y *YubiKey) checkOrSetPIN(ctx context.Context, prompt HardwareKeyPrompt) error {
	pin, err := prompt.AskPIN(ctx, PINOptional, KeyInfo{})
	if err != nil {
		return trace.Wrap(err)
	}

	switch pin {
	case piv.DefaultPIN:
		fmt.Fprintf(os.Stderr, "The default PIN %q is not supported.\n", piv.DefaultPIN)
		fallthrough
	case "":
		if pin, err = y.setPINAndPUKFromDefault(ctx, prompt); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(y.verifyPIN(pin))
}

type sharedPIVConnection struct {
	// card is a reader name used to find and connect to this yubiKey.
	// This value may change between OS's, or with other system changes.
	card string

	// conn is the shared PIV connection.
	conn              *piv.YubiKey
	mu                sync.Mutex
	activeConnections int
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

func (c *sharedPIVConnection) privateKey(slot piv.Slot, public crypto.PublicKey, auth piv.KeyAuth) (crypto.PrivateKey, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	privateKey, err := c.conn.PrivateKey(slot, public, auth)
	return privateKey, trace.Wrap(err)
}

func (c *sharedPIVConnection) getSerialNumber() (uint32, error) {
	release, err := c.connect()
	if err != nil {
		return 0, trace.Wrap(err)
	}
	defer release()
	serial, err := c.conn.Serial()
	return serial, trace.Wrap(err)
}

func (c *sharedPIVConnection) getVersion() (piv.Version, error) {
	release, err := c.connect()
	if err != nil {
		return piv.Version{}, trace.Wrap(err)
	}
	defer release()
	return c.conn.Version(), nil
}

func (c *sharedPIVConnection) reset() error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return trace.Wrap(c.conn.Reset())
}

func (c *sharedPIVConnection) setCertificate(key [24]byte, slot piv.Slot, cert *x509.Certificate) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return trace.Wrap(c.conn.SetCertificate(key, slot, cert))
}

func (c *sharedPIVConnection) certificate(slot piv.Slot) (*x509.Certificate, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	cert, err := c.conn.Certificate(slot)
	return cert, trace.Wrap(err)
}

func (c *sharedPIVConnection) generateKey(key [24]byte, slot piv.Slot, opts piv.Key) (crypto.PublicKey, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	pubKey, err := c.conn.GenerateKey(key, slot, opts)
	return pubKey, trace.Wrap(err)
}

func (c *sharedPIVConnection) attest(slot piv.Slot) (*x509.Certificate, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	cert, err := c.conn.Attest(slot)
	return cert, trace.Wrap(err)
}

func (c *sharedPIVConnection) attestationCertificate() (*x509.Certificate, error) {
	release, err := c.connect()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer release()
	cert, err := c.conn.AttestationCertificate()
	return cert, trace.Wrap(err)
}

func (c *sharedPIVConnection) setPIN(oldPIN string, newPIN string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return trace.Wrap(c.conn.SetPIN(oldPIN, newPIN))
}

func (c *sharedPIVConnection) setPUK(oldPUK string, newPUK string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return trace.Wrap(c.conn.SetPUK(oldPUK, newPUK))
}

func (c *sharedPIVConnection) unblock(puk string, newPIN string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return trace.Wrap(c.conn.Unblock(puk, newPIN))
}

func (c *sharedPIVConnection) verifyPIN(pin string) error {
	release, err := c.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()
	return trace.Wrap(c.conn.VerifyPIN(pin))
}

func (c *sharedPIVConnection) setPINAndPUKFromDefault(ctx context.Context, prompt HardwareKeyPrompt) (string, error) {
	pinAndPUK, err := prompt.ChangePIN(ctx, KeyInfo{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	// YubiKey requires that PIN and PUK be 6-8 characters.
	// Verify that we get valid values from the prompt.
	if !isPINLengthValid(pinAndPUK.PIN) {
		return "", trace.BadParameter("PIN must be 6-8 characters long")
	}
	if pinAndPUK.PIN == piv.DefaultPIN {
		return "", trace.BadParameter("The default PIN is not supported")
	}
	if !isPINLengthValid(pinAndPUK.PUK) {
		return "", trace.BadParameter("PUK must be 6-8 characters long")
	}
	if pinAndPUK.PUK == piv.DefaultPUK {
		return "", trace.BadParameter("The default PUK is not supported")
	}

	if pinAndPUK.PUKChanged {
		if err := c.setPUK(piv.DefaultPUK, pinAndPUK.PUK); err != nil {
			return "", trace.Wrap(err)
		}
	}

	if err := c.unblock(pinAndPUK.PUK, pinAndPUK.PIN); err != nil {
		return "", trace.Wrap(err)
	}

	return pinAndPUK.PIN, nil
}

func isRetryError(err error) bool {
	const retryError = "connecting to smart card: the smart card cannot be accessed because of other connections outstanding"
	return strings.Contains(err.Error(), retryError)
}

func (s PIVSlot) validate() error {
	_, err := s.parse()
	return trace.Wrap(err)
}

func (s PIVSlot) parse() (piv.Slot, error) {
	slotKey, err := strconv.ParseUint(string(s), 16, 32)
	if err != nil {
		return piv.Slot{}, trace.Wrap(err)
	}

	return parsePIVSlot(uint32(slotKey))
}

func parsePIVSlot(slotKey uint32) (piv.Slot, error) {
	switch slotKey {
	case piv.SlotAuthentication.Key:
		return piv.SlotAuthentication, nil
	case piv.SlotSignature.Key:
		return piv.SlotSignature, nil
	case piv.SlotKeyManagement.Key:
		return piv.SlotKeyManagement, nil
	case piv.SlotCardAuthentication.Key:
		return piv.SlotCardAuthentication, nil
	default:
		retiredSlot, ok := piv.RetiredKeyManagementSlot(slotKey)
		if !ok {
			return piv.Slot{}, trace.BadParameter("slot %X does not exist", slotKey)
		}
		return retiredSlot, nil
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
