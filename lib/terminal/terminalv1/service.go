// Copyright 2021 Gravitational, Inc
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

package terminalv1

import terminalpb "github.com/gravitational/teleport/protogen/teleport/terminal/v1"

// Service implements teleport.terminal.v1.TerminalService.
type Service struct {
	terminalpb.UnimplementedTerminalServiceServer
}

// TODO(codingllama): RPCs implementations go here.
//  Consider splitting files by resource so we don't end up with the entire system in a single file.
