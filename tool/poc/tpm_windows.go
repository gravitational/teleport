package main

import (
	"github.com/google/go-attestation/attest"
	"github.com/gravitational/trace"
)

func simulatedTPM() (*attest.TPM, error) {
	return nil, trace.BadParameter("not supported")
}
