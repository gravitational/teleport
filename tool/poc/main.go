package main

import (
	"bytes"
	"crypto/rand"
	"github.com/google/go-attestation/attest"
	"github.com/google/go-tpm-tools/simulator"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

func run() error {
	logger := utils.NewLogger()
	sim, err := simulator.Get()
	if err != nil {
		return trace.Wrap(err)
	}

	tpm := attest.InjectSimulatedTPMForTest(sim)

	eks, err := tpm.EKs()
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Printf("%+v", eks)
	ek := eks[0]
	logger.Printf("ek %+v", ek)

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
		EK:         ek,
		AK:         attestationParams,
	}
	solution, encryptedCredentials, err := activationParams.Generate()
	if err != nil {
		return trace.Wrap(err)
	}

	// Generate a nonce to use for attesting platform
	nonce := make([]byte, 128)
	_, err = rand.Read(nonce)
	if err != nil {
		return trace.Wrap(err)
	}

	// BACK ON CLIENT
	foundSolution, err := ak.ActivateCredential(tpm, *encryptedCredentials)
	if err != nil {
		return trace.Wrap(err)
	}

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
	// Check platform attestation
	akPub, err := attest.ParseAKPublic(attest.TPMVersion20, attestationParams.Public)
	if err != nil {
		return trace.Wrap(err)
	}
	err = akPub.VerifyAll(platformsParams.Quotes, platformsParams.PCRs, nonce)
	if err != nil {
		return trace.Wrap(err)
	}
	// Woohoo :D

	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
