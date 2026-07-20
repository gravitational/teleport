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
// CreatePairedDeviceEnrollToken RPC against a proxy server with a given
// pairing token and constant fake device data.
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
	token := flag.String("token", "", "enroll pairing token")
	flag.Parse()

	if *proxyServer == "" || *token == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := enroll.NewClient(*proxyServer, false)
	enrollToken, err := client.CreatePairedDeviceEnrollToken(*token, &fakeDeviceData)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(enrollToken.Token)
}
