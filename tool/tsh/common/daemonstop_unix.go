//go:build !windows
// +build !windows

// Copyright 2023 Gravitational, Inc
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

package common

// onDaemonStop implements the "tsh daemon stop" command. It handles graceful shutdown of the daemon
// on Windows, so it's a noop on other platforms. See daemonstop_windows.go for more details.
func onDaemonStop(cf *CLIConf) error {
	return nil
}
