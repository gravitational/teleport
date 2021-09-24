// Copyright 2021 Gravitational, Inc
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

//+build desktop_access_beta

package main

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/net/websocket"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/srv/desktop/deskproto"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
)

func main() {
	if len(os.Args) < 5 {
		log.Fatalf("usage: %s host:port user cert-pem-file key-pem-file", os.Args[0])
	}
	addr := os.Args[1]
	username := os.Args[2]
	userCertPath := os.Args[3]
	userKeyPath := os.Args[4]
	log.Printf("target addr: %q, username: %q, cert file: %q, key file: %q", addr, username, userCertPath, userKeyPath)

	userCertDER, err := decodePEMFile(userCertPath)
	if err != nil {
		log.Fatal(err)
	}
	userKeyDER, err := decodePEMFile(userKeyPath)
	if err != nil {
		log.Fatal(err)
	}
	// Convert form PKCS#8 to PKCS#1.
	userKey, err := x509.ParsePKCS8PrivateKey(userKeyDER)
	if err != nil {
		log.Fatal(err)
	}
	userKeyRSA, ok := userKey.(*rsa.PrivateKey)
	if !ok {
		log.Fatalf("private key is %T, expected and RSA key", userKey)
	}
	userKeyDER = x509.MarshalPKCS1PrivateKey(userKeyRSA)

	assetPath := filepath.Join(exeDir(), "testclient")
	log.Printf("serving assets from %q", assetPath)
	http.Handle("/", http.FileServer(http.Dir(assetPath)))
	http.Handle("/connect", handleConnect(addr, username, userCertDER, userKeyDER))

	log.Println("listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe failed:", err)
	}
}

func handleConnect(addr, username string, userCertDER, userKeyDER []byte) http.Handler {
	return websocket.Handler(func(conn *websocket.Conn) {
		log.Println("new websocket connection from", conn.RemoteAddr())
		defer log.Println("websocket connection from", conn.RemoteAddr(), "closed")
		usernameSent := false
		logger := logrus.New()
		logger.Level = logrus.DebugLevel
		c, err := rdpclient.New(context.Background(), rdpclient.Config{
			Addr: addr,
			GenerateUserCert: func(ctx context.Context, username string) (certDER, keyDER []byte, err error) {
				return userCertDER, userKeyDER, nil
			},
			OutputMessage: func(m deskproto.Message) error {
				data, err := m.Encode()
				if err != nil {
					return fmt.Errorf("failed to encode output message: %w", err)
				}
				return websocket.Message.Send(conn, data)
			},
			InputMessage: func() (deskproto.Message, error) {
				// Inject username as the first message.
				if !usernameSent {
					usernameSent = true
					return deskproto.ClientUsername{Username: username}, nil
				}
				var buf []byte
				if err := websocket.Message.Receive(conn, &buf); err != nil {
					return nil, fmt.Errorf("failed to read input message: %w", err)
				}
				return deskproto.Decode(buf)
			},
			Log: logger,
		})
		if err != nil {
			log.Printf("failed to create rdpclient: %v", err)
			return
		}
		if err := c.Wait(); err != nil {
			log.Printf("failed to wait for rdpclient to finish: %v", err)
			return
		}
	})
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		log.Println("failed to find executable path:", err)
		return "."
	}
	return filepath.Dir(exe)
}

func decodePEMFile(path string) ([]byte, error) {
	pemData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, rest := pem.Decode(pemData)
	if b == nil || len(b.Bytes) == 0 {
		return nil, fmt.Errorf("no valid PEM data in %q", path)
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		return nil, fmt.Errorf("trailing data in %q", path)
	}
	return b.Bytes, nil
}
