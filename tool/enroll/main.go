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
	rpc := flag.String("rpc", "create-token", "RPC to call: create-token or enroll")
	token := flag.String("token", "", "enroll pairing token for create-token, device enrollment token for enroll")
	user := flag.String("user", "", "owner to assign to the device (enroll only)")
	flag.Parse()

	if *proxyServer == "" || *token == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := enroll.NewClient(*proxyServer, false)

	switch *rpc {
	case "create-token":
		enrollToken, err := client.CreatePairedDeviceEnrollToken(*token, &fakeDeviceData)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(enrollToken.Token)
	case "enroll":
		if *user == "" {
			fmt.Fprintln(os.Stderr, "-user is required for enroll")
			os.Exit(1)
		}
		device, err := client.EnrollDevice(*token, *user, &fakeDeviceData)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("enrolled device %s (asset tag %s)\n", device.DeviceID, device.AssetTag)
	default:
		fmt.Fprintf(os.Stderr, "unknown rpc %q\n", *rpc)
		flag.Usage()
		os.Exit(1)
	}
}
