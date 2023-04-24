package main

import (
	"bytes"
	"crypto/rand"
	"github.com/sirupsen/logrus"
	"io"

	"github.com/google/go-attestation/attest"
	"github.com/google/go-tpm/tpm2"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const useSimulator = false

const useDifferentAK = false

// TODO: Determine what value this has
type windowsCmdChannel struct {
	io.ReadWriteCloser
}

func (cc *windowsCmdChannel) MeasurementLog() ([]byte, error) {
	return nil, nil
}

type server struct {
	storedAK *attest.AKPublic
	log      logrus.FieldLogger
}

type enrollChallenge func(challenge *attest.EncryptedCredential, platformAttestationNonce []byte) ([]byte, *attest.PlatformParameters, error)

func (s *server) Enroll(ek attest.EK, attestationParams attest.AttestationParameters, callback enrollChallenge) error {
	// TODO: IRL we would validate EK has trusted cert first.

	activationParams := attest.ActivationParameters{
		TPMVersion: attest.TPMVersion20,
		EK:         ek.Public,
		AK:         attestationParams,
	}
	realSolution, encryptedCredentials, err := activationParams.Generate()
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.Infof("generated activatation challenge, solution: %s", realSolution)

	// Generate a nonce to use for attesting platform
	nonce := make([]byte, 32)
	_, err = rand.Read(nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	clientSolution, platformParams, err := callback(encryptedCredentials, nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	if !bytes.Equal(clientSolution, realSolution) {
		return trace.BadParameter("incorrect solution")
	}
	s.log.Infof("passed credential activation check")
	// Check platform attestation
	akPub, err := attest.ParseAKPublic(attest.TPMVersion20, attestationParams.Public)
	if err != nil {
		return trace.Wrap(err)
	}
	err = akPub.VerifyAll(platformParams.Quotes, platformParams.PCRs, nonce)
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.Infof("passed platofrm params verifyall")
	eventLog, err := attest.ParseEventLog(platformParams.EventLog)
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.Infof("parsed event log")
	events, err := eventLog.Verify(platformParams.PCRs)
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.Infof("verified event log, entries: %d", len(events))
	sbs, err := attest.ParseSecurebootState(events)
	if err != nil {
		return trace.Wrap(err)
	}
	s.log.Infof("Secure boot state parsed %s", sbs.Enabled)

	s.storedAK = akPub
	return nil
}

type authenticateChallenge func(platformAttestationNonce []byte) (*attest.PlatformParameters, error)

func (s *server) Authenticate(callback authenticateChallenge) error {
	// Generate a nonce to use for attesting platform
	nonce := make([]byte, 32)
	_, err := rand.Read(nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	platformParams, err := callback(nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.storedAK.VerifyAll(platformParams.Quotes, platformParams.PCRs, nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func run(rootLog logrus.FieldLogger) error {
	var tpm *attest.TPM
	var err error
	if useSimulator {
		tpm, err = simulatedTPM()
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		_, err := tpm2.OpenTPM()
		if err != nil {
			return trace.Wrap(err)
		}
		openCfg := &attest.OpenConfig{
			TPMVersion: attest.TPMVersion20,
			// CommandChannel: &windowsCmdChannel{rawConn},
		}
		tpm, err = attest.OpenTPM(openCfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	srv := server{
		log: rootLog.WithField(trace.Component, "SERVER"),
	}
	logger := rootLog.WithField(trace.Component, "CLIENT")

	eks, err := tpm.EKs()
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Printf("%+v", eks)
	ek := eks[0]
	logger.Printf("ek %+v %T", ek, ek.Public)

	akConfig := &attest.AKConfig{}
	ak, err := tpm.NewAK(akConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	attestationParams := ak.AttestationParameters()

	err = srv.Enroll(ek, attestationParams, func(challenge *attest.EncryptedCredential, platformAttestationNonce []byte) ([]byte, *attest.PlatformParameters, error) {
		foundSolution, err := ak.ActivateCredential(tpm, *challenge)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		logger.Infof("activating credentials found solution: %s", foundSolution)

		platformsParams, err := tpm.AttestPlatform(ak, platformAttestationNonce, &attest.PlatformAttestConfig{
			EventLog: nil,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return foundSolution, platformsParams, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	logger.Infof("Enrollment complete")
	logger.Infof("Trying re-authentication")

	// Used to inject a failure case that should occur if a different AK is
	// used.
	if useDifferentAK {
		ak, err = tpm.NewAK(akConfig)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = srv.Authenticate(func(platformAttestationNonce []byte) (*attest.PlatformParameters, error) {
		platformsParams, err := tpm.AttestPlatform(ak, platformAttestationNonce, &attest.PlatformAttestConfig{
			EventLog: nil,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return platformsParams, nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("Authentication complete")

	return nil
}

func main() {
	logger := utils.NewLogger()
	if err := run(logger); err != nil {
		logger.WithError(err).Fatalf("Bad thing happened")
	}
}
