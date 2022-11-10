/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/constants"
)

const generateKeys = 42

func main() {
	keys := make([]byte, 0)

	for i := 0; i < generateKeys; i++ {
		private, err := rsa.GenerateKey(rand.Reader, constants.RSAKeySize)
		if err != nil {
			log.Fatal(err)
		}

		block := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(private),
		}

		privatePEM := pem.EncodeToMemory(block)
		keys = append(keys, privatePEM...)
	}

	if err := os.WriteFile("keys.pem", keys, 0644); err != nil {
		log.Fatal(err)
	}
}
