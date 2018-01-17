/*
Copyright 2017 Maximilien Richer

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

package ansible

import (
	"encoding/json"
	"testing"

	"github.com/gravitational/teleport/lib/services"
)

var serverFixture = []services.Server{
	&services.ServerV2{
		Metadata: services.Metadata{
			Labels: map[string]string{
				"os": "gentoo",
			},
		},
		Spec: services.ServerSpecV2{
			Addr:     "198.145.29.83:22",
			Hostname: "kernel.org",
		},
	},
	&services.ServerV2{
		Metadata: services.Metadata{
			Labels: map[string]string{
				"os":   "coreos",
				"role": "database",
			},
		},
		Spec: services.ServerSpecV2{
			Addr:     "11.1.1.1:1212",
			Hostname: "coreos.local",
		},
	},
	&services.ServerV2{
		Metadata: services.Metadata{
			Labels: map[string]string{
				"os":   "plan9",
				"role": "database",
			},
		},
		Spec: services.ServerSpecV2{
			Addr:     "8.8.4.4:8988",
			Hostname: "g00gle.com",
		},
	},
}

<<<<<<< HEAD
func TestDynamicInventoryHost(t *testing.T) {
	jsonInventory, err := DynamicInventoryList(serverFixture)
=======
// A simple regression test to check for proper encoding
func TestMarshalInventoryHost(t *testing.T) {
	jsonInventory, err := MarshalInventory(serverFixture)
>>>>>>> 65207966... Rename DynamicInventory to MarshalInventory
	if err != nil {
		t.Error(err)
	}

	encodedJSON := `{"_meta":{"hostvars":{}},"os-coreos":{"hosts":["11.1.1.1"],"vars":{}},"os-gentoo":{"hosts":["198.145.29.83"],"vars":{}},"os-plan9":{"hosts":["8.8.4.4"],"vars":{}},"role-database":{"hosts":["11.1.1.1","8.8.4.4"],"vars":{}}}` + "\n"
	var i Inventory
	err = json.Unmarshal([]byte(encodedJSON), &i)
	if err != nil {
		t.Error("cannot unmarshal fixture, did the type change?")
	}
	if jsonInventory != encodedJSON {
		t.Errorf("mismatch in json output\nGiven: %sExpct: %s", jsonInventory, encodedJSON)
	}
}
