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
	"fmt"
	"strings"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Inventory matches the JSON struct needed for DynamicInventoryList
type Inventory map[string]interface{}

// Group gather hosts and variables common to them
type Group struct {
	Hosts []string          `json:"hosts"`
	Vars  map[string]string `json:"vars"`
}

// MarshalInventory returns a JSON-formated ouput compatible with Ansible --list flag
//
// The JSON output SHOULD HAVE the following format:
// ```json
// {
//     "group_name": {
//         "hosts": ["host1.example.com", "host2.example.com"],
//         "vars": {
//             "a": true
//         }
//     },
//     "_meta": {
//         "hostvars": {}
//     }
// }
// ```
func MarshalInventory(nodes []services.Server) (string, error) {
	hostsByLabels := bufferLabels(nodes)

	var inventory = make(map[string]interface{})
	for labelDashValue, hosts := range hostsByLabels {
		inventory[labelDashValue] = Group{
			Hosts: hosts,
			Vars:  make(map[string]string),
		}
	}

	// Meta is a special group with information on each host
	// this gonna become "_meta": { "hostvars": { "host": {"var": value}}}
	// so the 2 top level (Meta and Hotvars) have only one key to match the struct
	// yes, the type is stupid, but blame python devs not me
	type Meta map[string]map[string]map[string]string
	meta := make(Meta)
	meta["hostvars"] = make(map[string]map[string]string)
	inventory["_meta"] = meta
	out, err := json.Marshal(inventory)
	if err != nil {
		return "", trace.Wrap(err, "can not encode JSON object")
	}
	return string(out) + "\n", nil
}

// MarshalInventoryHost returns a JSON-formated ouput compatible with Ansible --host <string> flag
//
// (From ansible ref. doc)
// When called with the arguments --host <hostname>, the script must print either an empty JSON hash/dictionary,
// or a hash/dictionary of variables to make available to templates and playbooks.
func MarshalInventoryHost(nodes []services.Server, host string) {
	// print an empty dic
	fmt.Print("{\"\":\"\"}\n")
}

// StaticInventory write to stdout an INI-formated ouput compatible with Ansible static inventory format
//
// It crafts groups using the labels associated with each nodes. Each label is build in the form
// <label>-<value> (with a dash in the middle).
func StaticInventory(nodes []services.Server) {
	inventory := bufferLabels(nodes)
	// write one tulpe by keys
	for groupName, nodeIPs := range inventory {
		fmt.Println("[" + groupName + "]")
		for _, IP := range nodeIPs {
			fmt.Println(IP)
		}
	}
}

// bufferLabels gather labels values and create groups associating hosts with identical labels values
func bufferLabels(nodes []services.Server) map[string][]string {
	labelBuffer := make(map[string][]string)
	// get all keys
	for _, n := range nodes {
		// get labels and add to groups
		for label, val := range n.GetAllLabels() {
			if val != "" {
				// groupName is of the form apache-2.2
				groupName := label + "-" + val
				// remove trailing port in host (if any)
				IP := trimTrailingPort(n.GetAddr())
				labelBuffer[groupName] = append(labelBuffer[groupName], IP)
			}
		}
	}
	return labelBuffer
}

func trimTrailingPort(nodeAddr string) (nodeIP string) {
	nodeIP = strings.Split(nodeAddr, ":")[0]
	return
}
