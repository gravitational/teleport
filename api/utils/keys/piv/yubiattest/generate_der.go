//go:build ignore

// Copyright 2026 Gravitational, Inc.
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

package main

import (
	"crypto/x509"
	"encoding/pem"
	"os"
)

func main() {
	convert("yubico-ca-1.pem", "yubico-ca-1.der")
	convert("yubico-fido-ca-1.pem", "yubico-fido-ca-1.der")
	convert("yubico-piv-ca-1.pem", "yubico-piv-ca-1.der")
	convert("yubico-intermediate.pem", "yubico-intermediate.der")
}

func convert(pemFile, derFile string) {
	pems, err := os.ReadFile(pemFile)
	if err != nil {
		panic(err)
	}
	var ders []byte

	for len(pems) > 0 {
		var b *pem.Block
		b, pems = pem.Decode(pems)
		if b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(b.Bytes)
		if err != nil {
			panic(err)
		}
		ders = append(ders, cert.Raw...)
	}

	if err := os.WriteFile(derFile, ders, 0o644); err != nil {
		panic(err)
	}
}
