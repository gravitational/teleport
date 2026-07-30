// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Command enroll is a throwaway tool that calls the public Device Trust
// CreatePairedDeviceEnrollToken and EnrollDevice RPCs against a proxy server
// with a given token and constant fake device data.
//
// The device described by fakeDeviceData must already be in the inventory:
//
//	tctl devices add --os ios --asset-tag FAKE000SERIAL
//
// "all" runs the flow the mobile app performs after scanning the QR code,
// taking the enroll pairing token from the Enroll Mobile Device wizard through
// to an enrolled device:
//
//	go run ./tool/enroll -proxy <host:port> -rpc all -token <enroll pairing token>
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/gravitational/teleport/lib/mobile/verify/enroll"
)

// fakeDeviceData is constant stand-in device data for manual testing.
var fakeDeviceData = enroll.DeviceCollectedData{
	SerialNumber:    "FAKE000SERIAL",
	ModelIdentifier: "iPhone16,1",
	VersionOS:       "18.0",
	BuildOS:         "22A000",
}

func main() {
	proxyServer := flag.String("proxy", "", "proxy server address (host:port)")
	rpc := flag.String("rpc", "create-token", "what to run: create-token, enroll, or all (create-token followed by enroll)")
	token := flag.String("token", "", "enroll pairing token for create-token and all, device enrollment token for enroll")
	flag.Parse()

	if *proxyServer == "" || *token == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := enroll.NewClient(*proxyServer, false)

	switch *rpc {
	case "create-token":
		enrollToken, err := createToken(client, *token)
		if err != nil {
			fail(err)
		}
		fmt.Println(enrollToken)
	case "enroll":
		if err := enrollDevice(client, *token); err != nil {
			fail(err)
		}
	case "all":
		enrollToken, err := createToken(client, *token)
		if err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "Got enrollment token %s, enrolling the device.\n", enrollToken)

		if err := enrollDevice(client, enrollToken); err != nil {
			fail(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown rpc %q\n", *rpc)
		flag.Usage()
		os.Exit(1)
	}
}

// createToken exchanges an enroll pairing token for a device enrollment token.
// The call blocks until the pairing is approved in the Web UI, so it announces
// itself rather than looking hung.
func createToken(client *enroll.Client, pairingToken string) (string, error) {
	fmt.Fprintln(os.Stderr, "Waiting for the request to be approved under Account Settings in the Web UI.")

	enrollToken, err := client.CreatePairedDeviceEnrollToken(pairingToken, &fakeDeviceData)
	if err != nil {
		return "", err
	}
	return enrollToken.Token, nil
}

func enrollDevice(client *enroll.Client, enrollToken string) error {
	device, err := client.EnrollDevice(enrollToken, &fakeDeviceData)
	if err != nil {
		return err
	}

	fmt.Printf("enrolled device %s (asset tag %s)\n", device.DeviceID, device.AssetTag)
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
