//go:build libfido2

/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

// This tool is used to load-test Teleport logins using the passwordless flow.
// It works by creating a software backed webauth device, and crafting a
// a Teleport client that uses it instead of real webauthn hardware tokens.
// This creates shareable webauthn resident credentials. This is absolutely
// unsafe and must never be used by anyone in production as this nullifies any
// webauthn security property.
//
// DO NOT USE THIS IN YOUR REAL ACCOUNT.
// Create a dedicated test account with limited privileges in a load-test cluster.
//
// For the sake of simplicity, only clusters with TLS routing are supported.
//
// Three subcommands are supported
//
//	generate: uses your local `.tsh/` profile to log in, do a MFA challenge,
//		and register a fake webauthn device.
//	once: loads a fake webauthn device and use it to log in
//	load: continuously login, performs 1000 logins, at the rate of ~20 logins per minute.
//
// Note: you might want to check
//   - that you are at least hitting 2 proxies, ideally 3, from the same IP, else you'll
//     get rate-limited (current login time is ~2 seconds), proxy rate limits between 20
//     and 40 log ins per minute.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/lib/auth/authtest" //nolint:depguard // It's safe to import authtest because we are in a standalone CLI, not in Teleport.
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

const usage = `Usage: login --proxy-addr=teleport.example.com (generate|once|load) <path-to-json>`

func main() {
	ctx := context.Background()
	var proxyAddr string
	var deviceName string
	flag.StringVar(&proxyAddr, "proxy-addr", "", "The Teleport Proxy address (host:port).")
	flag.StringVar(&deviceName, "device-name", "fake-mfa-device", "The MFA device name. Only used by `generate`.")

	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		fmt.Println(usage)
		os.Exit(1)
	}

	if proxyAddr == "" {
		log.Fatal("You must specify a Teleport Proxy address.")
	}

	err := run(ctx, args, proxyAddr, deviceName)
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string, proxyAddr, deviceName string) error {
	switch args[0] {
	case "generate":
		if deviceName == "" {
			return trace.BadParameter("missing device name")
		}
		tc, err := loadExistingClient(ctx, proxyAddr)
		if err != nil {
			return trace.Wrap(err, "loading existing client")
		}

		device, err := createAndRegisterFakeWebauthnDevice(ctx, tc, deviceName, proxyAddr)
		if err != nil {
			return trace.Wrap(err, "registering fake webauthn device")
		}

		_, err = loginWithFakeDevice(ctx, device, proxyAddr)
		if err != nil {
			return trace.Wrap(err, "login with fake device")
		}

		dump, err := json.Marshal(device)
		if err != nil {
			return trace.Wrap(err, "marshaling device")
		}
		if err := os.WriteFile(args[1], dump, 0600); err != nil {
			return trace.Wrap(err, "writing device json")
		}
		fmt.Printf("Successfully wrote device file %q.\n", args[1])
	case "once":
		device, err := readDeviceFromFile(args[1])
		if err != nil {
			return trace.Wrap(err, "reading device from file")
		}
		_, err = loginWithFakeDevice(ctx, device, proxyAddr)
		if err != nil {
			return trace.Wrap(err, "login with fake device")
		}
		fmt.Printf("Successfully logged in using device file %q.\n", args[1])
	case "load-test":
		if err := loadTest(ctx, args[1], proxyAddr); err != nil {
			return trace.Wrap(err, "load testing")
		}
	default:
		fmt.Println(usage)
		return trace.BadParameter("unknown command %v", args[0])
	}

	return nil
}

func readDeviceFromFile(path string) (*authtest.Device, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var device authtest.Device
	if err := json.Unmarshal(content, &device); err != nil {
		return nil, trace.Wrap(err)
	}

	return &device, nil
}

func loadExistingClient(ctx context.Context, proxyAddr string) (*client.TeleportClient, error) {
	clientStore := client.NewFSClientStore("")
	cfg := &client.Config{
		ClientStore: clientStore,
	}

	if err := cfg.LoadProfile(proxyAddr); err != nil {
		return nil, trace.Wrap(err, "loading profile")
	}

	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating client")
	}
	tc.AuthenticatorAttachment = wancli.AttachmentCrossPlatform

	// Checking that the client is OK
	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "connecting to cluster")
	}
	authClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = authClient.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

func createAndRegisterFakeWebauthnDevice(ctx context.Context, tc *client.TeleportClient, deviceName, proxyAddr string) (*authtest.Device, error) {
	// We pass the MFA ceremony with our existing client to be allowed to add devices.
	mfaResp, err := tc.NewMFACeremony().Run(ctx, &proto.CreateAuthenticateChallengeRequest{
		ChallengeExtensions: &mfav1.ChallengeExtensions{
			Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_MANAGE_DEVICES,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterClient, err := tc.ConnectToCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "creating cluster client")
	}
	defer clusterClient.Close()
	rootAuthClient, err := clusterClient.ConnectToRootCluster(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rootAuthClient.Close()

	// We create and register the fake webauthn device.
	const (
		devType  = proto.DeviceType_DEVICE_TYPE_WEBAUTHN
		devUsage = proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS
	)
	// Issue the registration challenge.
	registerChallenge, err := rootAuthClient.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		ExistingMFAResponse: mfaResp,
		DeviceType:          devType,
		DeviceUsage:         devUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u := &url.URL{
		Scheme: "https",
		Host:   proxyAddr,
	}

	dev, registerSolved, err := authtest.NewTestDeviceFromChallenge(
		registerChallenge,
		authtest.WithPasswordless(),
		authtest.WithOrigin(u.String()),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating device and solving registration channel")
	}

	addResp, err := rootAuthClient.AddMFADeviceSync(ctx, &proto.AddMFADeviceSyncRequest{
		NewDeviceName:  deviceName,
		NewMFAResponse: registerSolved,
		DeviceUsage:    devUsage,
	})
	if err != nil {
		return nil, trace.Wrap(err, "adding device")
	}
	dev.MFA = addResp.Device

	log.Printf("Registered test device %q for user %q", deviceName, tc.Username)
	return dev, nil
}

func loginWithFakeDevice(ctx context.Context, device *authtest.Device, proxyAddr string) (*client.TeleportClient, error) {
	// Little hack to reset the counter. Signature counters are optional in webauthn
	// because shared passkeys (i.e. password managers) cannot guarantee strict monotony.
	// Teleport always allow login, even in case of counter mismatch, but will complain
	// in the logs. Setting it to zero avoids this.
	device.Key.SetCounter(0)

	newMFAPrompt := func(cfg *libmfa.PromptConfig) mfa.Prompt {
		return fakePrompt{dev: device}
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	webauthnLogin := func(
		ctx context.Context,
		origin string,
		assertion *wantypes.CredentialAssertion,
		prompt wancli.LoginPrompt,
		opts *wancli.LoginOpts,
	) (*proto.MFAAuthenticateResponse, string, error) {
		a := wantypes.CredentialAssertionToProto(assertion)
		rsp, err := device.SolveAuthn(&proto.MFAAuthenticateChallenge{
			WebauthnChallenge: a,
			MFARequired:       proto.MFARequired_MFA_REQUIRED_YES,
		})
		if err != nil {
			return nil, "", trace.Wrap(err, "solving authn")
		}
		return rsp, "test", nil
	}

	store := client.NewMemClientStore()
	cfg := &client.Config{
		Username:             "test",
		Host:                 proxyAddr,
		WebProxyAddr:         proxyAddr,
		ClientStore:          store,
		Stdout:               os.Stdout,
		Stderr:               os.Stderr,
		Stdin:                os.Stdin,
		AuthConnector:        "passwordless",
		TLSRoutingEnabled:    true,
		WebauthnLogin:        webauthnLogin,
		MFAPromptConstructor: newMFAPrompt,
		StdinFunc:            nil,
		// Don't try to load stuff in the local SSH agent,
		// else it will die because of the number of keys.
		AddKeysToAgent: client.AddKeysToAgentNo,
	}
	tc, err := client.NewClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err, "creating client")
	}
	ring, err := tc.Login(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "logging in")
	}

	// Checking that the client is OK.
	_, authClient, err := tc.ConnectToRootCluster(ctx, ring)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := authClient.Ping(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return tc, nil
}

type fakePrompt struct {
	mfa.Prompt

	dev *authtest.Device
}

func (f fakePrompt) Run(_ context.Context, chal *proto.MFAAuthenticateChallenge) (*proto.MFAAuthenticateResponse, error) {
	return f.dev.SolveAuthn(chal)
}

func loadTest(ctx context.Context, path, proxyAddr string) error {
	maxLogins := 1000

	m := testMetrics{
		logins:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "logins_total", Help: "Total number of logins"}, []string{"result"}),
		loginTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "login_duration", Help: "Time taken to log in seconds"}, []string{"result"}),
	}
	r, err := metrics.NewRegistry(prometheus.DefaultRegisterer, "teleport", "loadtest")
	if err != nil {
		return trace.Wrap(err, "metrics.NewRegistry")
	}
	if err := m.register(r); err != nil {
		return trace.Wrap(err)
	}

	device, err := readDeviceFromFile(path)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Printf("Device %q loaded", device.MFA.Id)

	defer outputResults()

	// Rate limiting to 1 call every 3 sec to make sure we don't exceed 20
	// calls per minute which is the proxy login rate-limit per-IP.
	ticker := time.NewTicker(3 * time.Second)
	for range maxLogins {
		select {
		case <-ticker.C:
			start := time.Now()
			_, err := loginWithFakeDevice(ctx, device, proxyAddr)
			if err != nil {
				m.logins.With(prometheus.Labels{"result": "failure"}).Inc()
				m.loginTime.With(prometheus.Labels{"result": "failure"}).Observe(time.Since(start).Seconds())
				log.Printf("Failed with error: %q", err)
				continue
			}
			m.logins.With(prometheus.Labels{"result": "success"}).Inc()
			m.loginTime.With(prometheus.Labels{"result": "success"}).Observe(time.Since(start).Seconds())
		case <-ctx.Done():
			ticker.Stop()
			return trace.Wrap(ctx.Err())
		}
	}

	log.Println("Load-test done")
	return nil
}

func outputResults() {
	// Gather the prometheus metrics and yeet them into stdout.
	var b bytes.Buffer
	enc := expfmt.NewEncoder(&b, expfmt.NewFormat(expfmt.TypeTextPlain))
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		log.Printf("Error gathering metrics: %v", err)
		return
	}

	var errors []error
	for _, mf := range mfs {
		errors = append(errors, enc.Encode(mf))
	}
	if trace.NewAggregate(errors...) != nil {
		fmt.Printf("Error encoding results: %v\n", errors)
		return
	}
	log.Println(b.String())
}

type testMetrics struct {
	logins    *prometheus.CounterVec
	loginTime *prometheus.HistogramVec
}

func (m *testMetrics) register(r *metrics.Registry) error {
	return trace.NewAggregate(
		r.Register(m.logins),
		r.Register(m.loginTime),
	)
}
