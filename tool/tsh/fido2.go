// Copyright 2022 Gravitational, Inc
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
	"fmt"
	"os"

	"github.com/gravitational/trace"

	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

func onFIDO2Diag(cf *CLIConf) error {
	diag, err := wancli.FIDO2Diag(cf.Context, os.Stdout)
	// Abort if we got a nil diagnostic, otherwise print as much as we can.
	if diag == nil {
		return trace.Wrap(err)
	}

	fmt.Printf("\nFIDO2 available: %v\n", diag.Available)
	fmt.Printf("Register successful? %v\n", diag.RegisterSuccessful)
	fmt.Printf("Login successful? %v\n", diag.LoginSuccessful)
	if err != nil {
		fmt.Println()
	}

	return trace.Wrap(err)
}
