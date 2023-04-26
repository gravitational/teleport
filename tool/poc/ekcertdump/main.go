package main

import (
	"encoding/pem"
	"fmt"
	"github.com/google/go-attestation/attest"
	"log"
	"os"
)

func run() error {
	openCfg := &attest.OpenConfig{
		TPMVersion: attest.TPMVersion20,
	}
	tpm, err := attest.OpenTPM(openCfg)
	if err != nil {
		return err
	}

	eks, err := tpm.EKs()
	if err != nil {
		return err
	}

	for i, ek := range eks {
		f, err := os.Create(fmt.Sprintf("tpm-ekcert-%d.pem", i))
		if err != nil {
			return err
		}
		defer f.Close()
		if ek.Certificate != nil {
			pem.Encode(f, &pem.Block{
				Type:  "CERTIFICATE",
				Bytes: ek.Certificate.Raw,
			})
		} else {
			fmt.Fprintf(f, "No EKCert present, suggested url: %s", ek.CertificateURL)
		}
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Fatalf(err.Error())
	}
}
