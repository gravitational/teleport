package main

import (
	"bytes"
	"crypto/rand"
	"github.com/google/go-attestation/attest"
	"github.com/google/go-tpm-tools/simulator"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

var logger = utils.NewLogger()

const useSimulator = false

func run() error {
	var tpm *attest.TPM
	var err error
	if useSimulator {
		sim, err := simulator.Get()
		if err != nil {
			return trace.Wrap(err)
		}

		tpm = attest.InjectSimulatedTPMForTest(sim)
	} else {
		openCfg := &attest.OpenConfig{
			TPMVersion: attest.TPMVersion20,
		}
		tpm, err = attest.OpenTPM(openCfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}

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

	// SERVER
	// TODO: Validate EK
	activationParams := attest.ActivationParameters{
		TPMVersion: attest.TPMVersion20,
		EK:         ek.Public,
		AK:         attestationParams,
	}
	solution, encryptedCredentials, err := activationParams.Generate()
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("generated activatation challenge, solution: %s", solution)

	// Generate a nonce to use for attesting platform
	nonce := make([]byte, 32)
	_, err = rand.Read(nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	// BACK ON CLIENT
	foundSolution, err := ak.ActivateCredential(tpm, *encryptedCredentials)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("activating credentials found solution: %s", foundSolution)

	platformsParams, err := tpm.AttestPlatform(ak, nonce, &attest.PlatformAttestConfig{
		EventLog: nil,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// BACK ON SERVER
	// Check credential activation was valid
	if !bytes.Equal(foundSolution, solution) {
		return trace.BadParameter("incorrect solution")
	}
	logger.Infof("passed credential activation check")
	// Check platform attestation
	akPub, err := attest.ParseAKPublic(attest.TPMVersion20, attestationParams.Public)
	if err != nil {
		return trace.Wrap(err)
	}
	err = akPub.VerifyAll(platformsParams.Quotes, platformsParams.PCRs, nonce)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("passed platofrm params verifyall")
	eventLog, err := attest.ParseEventLog(platformsParams.EventLog)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("passed event log")
	sbs, err := attest.ParseSecurebootState(eventLog.Events(attest.HashSHA256))
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("Secure boot state parsed %s", sbs.Enabled)
	// Woohoo :D

	return nil
}

func main() {
	if err := run(); err != nil {
		logger.WithError(err).Fatalf("Bad thing happened")
	}
}
