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
	if len(os.Args) < 3 {
		log.Fatalf("usage: TELEPORT_DEV_RDP_PASSWORD=password %s host:port user", os.Args[0])
	}
	addr := os.Args[1]
	username := os.Args[2]
	if os.Getenv("TELEPORT_DEV_RDP_PASSWORD") == "" {
		log.Fatal("missing TELEPORT_DEV_RDP_PASSWORD env var")
	}
	log.Printf("target addr: %q, username: %q", addr, username)

	assetPath := filepath.Join(exeDir(), "testclient")
	log.Printf("serving assets from %q", assetPath)
	http.Handle("/", http.FileServer(http.Dir(assetPath)))
	http.Handle("/connect", handleConnect(addr, username))

	log.Println("listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe failed:", err)
	}
}

func handleConnect(addr, username string) http.Handler {
	return websocket.Handler(func(conn *websocket.Conn) {
		log.Println("new websocket connection from", conn.RemoteAddr())
		defer log.Println("websocket connection from", conn.RemoteAddr(), "closed")
		usernameSent := false
		logger := logrus.New()
		logger.Level = logrus.DebugLevel
		c, err := rdpclient.New(rdpclient.Config{
			Addr: addr,
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
