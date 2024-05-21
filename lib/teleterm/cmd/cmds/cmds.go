// Copyright 2024 Gravitational, Inc
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

package cmds

import (
	"os/exec"
)

// Cmds represents a single command in two variants – one that can be used to spawn a process and
// one that can be copied and pasted into a terminal.
//
// Defined in a separate package to avoid cyclic imports. CLI commands got refactored in v15+ anyway.
type Cmds struct {
	// Exec is the command that should be used when directly executing a command for the given
	// gateway.
	Exec *exec.Cmd
	// Preview is the command that should be used to display the command in the UI. Typically this
	// means that Preview includes quotes around special characters, so that the command gets executed
	// properly when the user copies and then pastes it into a terminal.
	Preview *exec.Cmd
}
