/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client/terminal"
	"github.com/gravitational/teleport/lib/kube/proxy/streamproto"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type KubeSession struct {
	stream    *streamproto.SessionStream
	terminal  *terminal.Terminal
	close     *utils.CloseBroadcaster
	closeWait *sync.WaitGroup
}

func NewKubeSession(ctx context.Context, tc *TeleportClient, meta types.Session, key *Key, kubeAddr string, tlsServer string) (*KubeSession, error) {
	close := utils.NewCloseBroadcaster()
	closeWait := &sync.WaitGroup{}
	joinEndpoint := "wss://" + kubeAddr + "/api/v1/teleport/join/" + meta.GetID()
	kubeCluster := meta.GetKubeCluster()
	ciphers := utils.DefaultCipherSuites()
	tlsConfig, err := key.KubeClientTLSConfig(ciphers, kubeCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tlsServer != "" {
		tlsConfig.ServerName = tlsServer
	}

	dialer := &websocket.Dialer{
		TLSClientConfig: tlsConfig,
	}

	ws, resp, err := dialer.Dial(joinEndpoint, nil)
	if err != nil {
		body, _ := ioutil.ReadAll(resp.Body)
		bodyString := string(body)
		fmt.Printf("handshake failed with status %d\nand body: %v\n", resp.StatusCode, bodyString)
		return nil, trace.Wrap(err)
	}

	// TODO(joel): set correct terminal size and deal with terminal resizing

	stream := streamproto.NewSessionStream(ws)

	terminal, err := terminal.New(tc.Stdin, tc.Stdout, tc.Stderr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	closeWait.Add(1)
	go func() {
		<-close.C
		terminal.Close()
		closeWait.Done()
	}()

	if terminal.IsAttached() {
		// Put the terminal into raw mode. Note that this must be done before
		// pipeInOut() as it may replace streams.
		terminal.InitRaw(true)
	}

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt)
	go func() {
		<-exit
		close.Close()
	}()

	s := &KubeSession{stream, terminal, close, closeWait}
	s.pipeInOut()
	return s, nil
}

func (s *KubeSession) pipeInOut() {
	go func() {
		defer s.close.Close()
		_, err := io.Copy(s.terminal.Stdout(), s.stream)
		if err != nil {
			fmt.Printf("error while reading remote stream: %v\n\r", err.Error())
		}
	}()

	go func() {
		defer s.close.Close()

		for {
			buf := make([]byte, 1)
			_, err := s.terminal.Stdin().Read(buf)
			if err == io.EOF {
				break
			}

			// Ctrl-C
			if buf[0] == '\x03' {
				fmt.Print("\n\rLeft session\n\r")
				break
			}

			// Ctrl-T
			if buf[0] == 't' {
				fmt.Print("\n\rForcefully terminated session\n\r")
				err := s.stream.DoForceTerminate()
				if err != nil {
					fmt.Printf("\n\rerror while sending force termination request: %v\n\r", err.Error())
				}

				break
			}
		}
	}()
}

func (s *KubeSession) Wait() {
	s.closeWait.Wait()
}

func (s *KubeSession) Close() {
	s.close.Close()
	s.closeWait.Wait()
}
