package devicetrustv1

import (
	"crypto"
	"crypto/rand"
	"crypto/subtle"

	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
)

func platformAttestationChallenge(
	akPublic []byte,
) (
	nonce []byte,
	finish func(platformParams attest.PlatformParameters) error,
	err error,
) {
	// Generate a nonce for the client to include in its attestation to prevent
	// replays of the attested quotes. Some TPMs do not support over 20 bytes
	// in the nonce.
	nonce = make([]byte, 20)
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, trace.Wrap(err, "generating nonce")
	}

	ak, err := attest.ParseAKPublic(
		attest.TPMVersion20,
		akPublic,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err, "parsing ak public")
	}

	return nonce, func(platformParams attest.PlatformParameters) error {
		// Now we validate the platform attestation provided by the device using
		// the nonce generated earlier and the known AK for the device.
		err := ak.VerifyAll(
			platformParams.Quotes,
			platformParams.PCRs,
			nonce,
		)
		if err != nil {
			return trace.Wrap(err, "verifying pcrs")
		}
		// We know now that the PCRs provided are legitimate and signed by the AK
		// and that our nonce was used in this process to prevent replay attacks.

		// Now we parse the event log, and replay it to see if it results in the
		// same state as currently presented by the PCRs. If this fails,
		// it indicates attempted tampering.
		eventLog, err := attest.ParseEventLog(platformParams.EventLog)
		if err != nil {
			return trace.Wrap(err, "parsing event log")
		}
		if _, err = eventLog.Verify(platformParams.PCRs); err != nil {
			return trace.Wrap(err, "verifying event log")
		}
		// We now have a "legitimate" event log, but there is some caveats.
		// See https://github.com/google/go-attestation/blob/master/docs/event-log-disclosure.md

		// TODO: Return PCRs for persistent in DCD for comparison in future
		// logins:
		// https://github.com/gravitational/teleport.e/issues/1421
		return nil
	}, nil
}

func credentialActivationChallenge(
	ek crypto.PublicKey,
	attestationParameters attest.AttestationParameters,
) (
	*attest.EncryptedCredential,
	func(clientSolution []byte) error,
	error,
) {
	activationParameters := attest.ActivationParameters{
		TPMVersion: attest.TPMVersion20,
		AK:         attestationParameters,
		EK:         ek,
	}
	// The generate method completes initial validation that provides the
	// following assurances:
	// - The attestation key is of a secure length
	// - The attestation key is marked as created within a TPM
	// - The attestation key is marked as restricted (e.g cannot be used to
	//   sign or decrypt external data)
	// When the returned challenge is solved by the TPM using ActivateCredential
	// the following additional assurance is given:
	// - The attestation key resides in the same TPM as the endorsement key
	solution, encryptedCredential, err := activationParameters.Generate()
	if err != nil {
		return nil, nil, trace.Wrap(err, "generating credential activation challenge")
	}
	return encryptedCredential, func(clientSolution []byte) error {
		if subtle.ConstantTimeCompare(clientSolution, solution) == 0 {
			return trace.BadParameter("invalid credential activation solution")
		}
		return nil
	}, nil
}
