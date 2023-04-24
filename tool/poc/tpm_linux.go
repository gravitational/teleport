package main

import (
	"github.com/google/go-attestation/attest"
	"github.com/google/go-tpm-tools/simulator"
	"github.com/gravitational/trace"
)

func simulatedTPM() (*attest.TPM, error) {
	sim, err := simulator.Get()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return attest.InjectSimulatedTPMForTest(sim), nil
}
