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
			Addr:     "198.145.29.83",
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
			Addr:     "11.1.1.1",
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
			Addr:     "8.8.4.4",
			Hostname: "g00gle.com",
		},
	},
}

func TestDynamicInventoryHost(t *testing.T) {
	jsonInventory, err := DynamicInventoryList(serverFixture)
	if err != nil {
		t.Error(err)
	}

	encodedJSON :=
		`{"Groups":{
			"os-coreos":{"Hosts":["11.1.1.1"],"Vars":{}},
			"os-gentoo":{"Hosts":["198.145.29.83"],"Vars":{}},
			"os-plan9":{"Hosts":["8.8.4.4"],"Vars":{}},
			"role-database":{"Hosts":["11.1.1.1","8.8.4.4"],"Vars":{}}
		}}`
	var i Inventory
	err = json.Unmarshal([]byte(encodedJSON), &i)
	if err != nil {
		t.Error("cannot unmarshal fixture, did the type change?")
	}
	reEncodedJSON, _ := json.Marshal(i)
	if jsonInventory != string(reEncodedJSON) {
		t.Errorf("mismatch in json output\nGiven: %s\nExpct: %s", jsonInventory, string(reEncodedJSON))
	}
}
