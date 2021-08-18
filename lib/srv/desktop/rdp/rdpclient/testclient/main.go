//+build desktop_access_beta

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/net/websocket"

	"github.com/gravitational/teleport/lib/srv/desktop/deskproto"
	"github.com/gravitational/teleport/lib/srv/desktop/rdp/rdpclient"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: %s host:port user password", os.Args[0])
	}
	addr := os.Args[1]
	username := os.Args[2]
	password := os.Args[3]

	assetPath := filepath.Join(exeDir(), "testclient")
	log.Printf("serving assets from %q", assetPath)
	http.Handle("/", http.FileServer(http.Dir(assetPath)))
	http.Handle("/connect", handleConnect(addr, username, password))

	log.Println("listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe failed:", err)
	}
}

func handleConnect(addr, username, password string) http.Handler {
	return websocket.Handler(func(conn *websocket.Conn) {
		done := make(chan struct{})
		defer close(done)
		inputPipe := make(chan deskproto.Message, 1)
		go func() {
			defer close(inputPipe)
			for {
				var buf []byte
				if err := websocket.Message.Receive(conn, &buf); err != nil {
					log.Printf("failed to read input message: %v", err)
					return
				}
				msg, err := deskproto.Decode(buf)
				if err != nil {
					log.Printf("failed to parse input message: %v", err)
					return
				}
				select {
				case inputPipe <- msg:
				case <-done:
					return
				}
			}
		}()

		c, err := rdpclient.New(rdpclient.Options{
			Addr: addr,
			OutputMessage: func(m deskproto.Message) error {
				if _, ok := m.(deskproto.UsernamePasswordRequired); ok {
					// Inject username/password response.
					select {
					case inputPipe <- deskproto.UsernamePasswordResponse{Username: username, Password: password}:
						return nil
					case <-done:
						return io.ErrClosedPipe
					}
				}
				data, err := m.Encode()
				if err != nil {
					return fmt.Errorf("failed to encode output message: %w", err)
				}
				return websocket.Message.Send(conn, data)
			},
			InputMessage: func() (deskproto.Message, error) {
				select {
				case msg, ok := <-inputPipe:
					if !ok {
						return nil, io.EOF
					}
					return msg, nil
				case <-done:
					return nil, io.EOF
				}
			},
		})
		if err != nil {
			log.Fatalf("failed to create rdpclient: %v", err)
		}
		if err := c.Wait(); err != nil {
			log.Fatalf("failed to wait for rdpclient to finish: %v", err)
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
