/*

 Copyright 2023 Gravitational, Inc.

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

package assist

import "github.com/gravitational/teleport/lib/ai"

// commandPayload is a payload for a command message.
type commandPayload struct {
	Command string     `json:"command,omitempty"`
	Nodes   []string   `json:"nodes,omitempty"`
	Labels  []ai.Label `json:"labels,omitempty"`
}

// partialMessagePayload is a payload for a partial message.
type partialMessagePayload struct {
	Content string `json:"content,omitempty"`
	Idx     int    `json:"idx,omitempty"`
}

// partialFinalizePayload is a payload for a partial finalize message.
type partialFinalizePayload struct {
	Idx int `json:"idx,omitempty"`
}
