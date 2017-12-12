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
	"fmt"

	"github.com/gravitational/teleport/lib/services"
)

// DynamicInventoryList returns a JSON-formated ouput compatible with Ansible --list flag
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
// }
// ```
func DynamicInventoryList(nodes []services.Server) {
	// this match the JSON struct needed for DynamicInventoryList
	type Group struct {
		hosts []string
		vars  map[string]string
	}
	type Inventory struct {
		groups map[string]Group
	}
	var inventory Inventory

	hostsByLabels := bufferLabels(nodes)

	for labelDashValue, hosts := range hostsByLabels {
		inventory.groups[labelDashValue] = Group{
			hosts: hosts,
			vars:  nil,
		}
	}
}

// DynamicInventoryHost returns a JSON-formated ouput compatible with Ansible --host <string> flag
//
// (From ansible ref. doc)
// When called with the arguments --host <hostname>, the script must print either an empty JSON hash/dictionary,
// or a hash/dictionary of variables to make available to templates and playbooks.
func DynamicInventoryHost(nodes []services.Server, host string) {
	// filter only the required node
}

// StaticInventory returns an INI-formated ouput compatible with Ansible static inventory format
//
// It crafts groups using the labels associated with each nodes. Each label is build in the form
// <label>-<value> (with a dash in the middle).
func StaticInventory(nodes []services.Server) {
	inventory := make(map[string][]string)
	// get all keys
	for _, n := range nodes {
		// get labels and add to groups
		for label, val := range n.GetAllLabels() {
			// groupName is of the form apache-2.2
			groupName := label + "-" + val
			inventory[groupName] = append(inventory[groupName], n.GetAddr())
		}
	}
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
			// groupName is of the form apache-2.2
			groupName := label + "-" + val
			labelBuffer[groupName] = append(labelBuffer[groupName], n.GetAddr())
		}
	}
	return labelBuffer
}
