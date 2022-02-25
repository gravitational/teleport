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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	libclient "github.com/gravitational/teleport/lib/client"

	_ "embed" // enable embed
)

//go:embed index.html
var indexPage []byte

var (
	addr     = flag.String("addr", "localhost:8080", "Server bind address")
	certFile = flag.String("cert_file", "cert.pem", "Cert PEM file")
	keyFile  = flag.String("key_file", "cert-key.pem", "Key PEM file")

	authAddr = flag.String("auth_addr", "localhost:3025", "Teleport Auth address")
	webAddr  = flag.String("web_addr", "localhost:3080", "Teleport Web API address")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Println("Starting Teleport client")
	profile := apiclient.LoadProfile("", "")
	teleport, err := apiclient.New(ctx, apiclient.Config{
		Addrs:       []string{*authAddr},
		Credentials: []apiclient.Credentials{profile},
	})
	if err != nil {
		fmt.Println("Teleport client startup failed, did you run tsh login?")
		return trace.Wrap(err)
	}

	http.Handle("/", http.RedirectHandler("/index.html", http.StatusSeeOther))
	http.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexPage)
	})

	s := &server{
		ctx:      ctx,
		teleport: teleport,
	}
	http.HandleFunc("/login/1", s.login1)
	http.HandleFunc("/login/2", s.login2)
	http.HandleFunc("/register/1", s.register1)
	http.HandleFunc("/register/2", s.register2)

	fmt.Printf("Listening at %v\n", *addr)
	return http.ListenAndServeTLS(*addr, *certFile, *keyFile, nil /* handler */)
}

type server struct {
	ctx      context.Context
	teleport *apiclient.Client

	mu                sync.Mutex
	inFlightAddStream *proto.AuthService_AddMFADeviceClient
}

type login1Request struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

func (s *server) login1(w http.ResponseWriter, r *http.Request) {
	var req login1Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get login challenge.
	body, err := json.Marshal(&libclient.MFAChallengeRequest{
		User: req.User,
		Pass: req.Pass,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp, err := http.Post("https://"+*webAddr+"/webapi/mfa/login/begin", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("INFO /mfa/login/begin: %#v", resp)
		http.Error(w, "Unexpected status from /mfa/login/begin", http.StatusBadRequest)
	}
	var challenge client.MFAAuthenticateChallenge
	if err := json.NewDecoder(resp.Body).Decode(&challenge); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if challenge.WebauthnChallenge == nil {
		http.Error(w, "nil credential assertion", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(challenge.WebauthnChallenge); err != nil {
		log.Println(err)
	}
}

type login2Request struct {
	wanlib.CredentialAssertionResponse
	User string `json:"user"`
}

func (s *server) login2(w http.ResponseWriter, r *http.Request) {
	// Request body is a wanlib.CredentialAssertionResponse.
	// User passed as a query param to make things simpler.

	var req login2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := json.Marshal(&libclient.AuthenticateWebUserRequest{
		User:                      req.User,
		WebauthnAssertionResponse: &req.CredentialAssertionResponse,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Solve login challenge
	resp, err := http.Post("https://"+*webAddr+"/webapi/mfa/login/finishsession", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("INFO /mfa/login/finishsession: %#v", resp)
		http.Error(w, "Unexpected status from /mfa/login/finishsession", http.StatusBadRequest)
	}

	// Login OK.
	w.WriteHeader(http.StatusOK)
}

type register1Request struct {
	User     string `json:"user"`
	Pass     string `json:"pass"`
	DevName  string `json:"dev_name"`
	TOTPCode string `json:"totp_code"`
}

func (s *server) register1(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock() // Hold lock for the entire time, we don't care.
	defer s.mu.Unlock()

	var req register1Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Init stream with device name and type.
	ctx := s.ctx
	stream, err := s.teleport.AddMFADevice(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := stream.Send(&proto.AddMFADeviceRequest{
		Request: &proto.AddMFADeviceRequest_Init{
			Init: &proto.AddMFADeviceRequestInit{
				DeviceName: req.DevName,
				DeviceType: proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			},
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Solve authn challenge.
	resp, err := stream.Recv()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var authResp *proto.MFAAuthenticateResponse
	challenge := resp.GetExistingMFAChallenge()
	switch {
	case challenge.GetTOTP() == nil && challenge.GetWebauthnChallenge() == nil: // aka empty challenge
		authResp = &proto.MFAAuthenticateResponse{}
	case challenge.GetTOTP() != nil:
		authResp = &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_TOTP{
				TOTP: &proto.TOTPResponse{
					Code: req.TOTPCode,
				},
			},
		}
	default:
		http.Error(w, "TOTP challenge not present", http.StatusBadRequest)
		return
	}
	if err := stream.Send(&proto.AddMFADeviceRequest{
		Request: &proto.AddMFADeviceRequest_ExistingMFAResponse{
			ExistingMFAResponse: authResp,
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err = stream.Recv()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ccProto := resp.GetNewMFARegisterChallenge().GetWebauthn()
	if ccProto == nil {
		http.Error(w, "nil credential creation", http.StatusBadRequest)
		return
	}

	cc := wanlib.CredentialCreationFromProto(ccProto)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(cc); err != nil {
		log.Println(err)
	}

	s.inFlightAddStream = &stream // Save stream for 2nd step
}

func (s *server) register2(w http.ResponseWriter, r *http.Request) {
	// Request body is a wanlib.CredentialCreationResponse.

	s.mu.Lock() // Hold lock for the entire time, we don't care.
	defer s.mu.Unlock()

	if s.inFlightAddStream == nil {
		http.Error(w, "In-flight add stream is nil", http.StatusBadRequest)
		return
	}
	stream := *s.inFlightAddStream

	var ccr wanlib.CredentialCreationResponse
	if err := json.NewDecoder(r.Body).Decode(&ccr); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Send register response.
	if err := stream.Send(&proto.AddMFADeviceRequest{
		Request: &proto.AddMFADeviceRequest_NewMFARegisterResponse{
			NewMFARegisterResponse: &proto.MFARegisterResponse{
				Response: &proto.MFARegisterResponse_Webauthn{
					Webauthn: wanlib.CredentialCreationResponseToProto(&ccr),
				},
			},
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := stream.Recv()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if resp.GetAck() == nil {
		log.Printf("WARN Expected Ack, got %#v", resp)
	}

	s.inFlightAddStream = nil
	w.WriteHeader(http.StatusOK)
}
