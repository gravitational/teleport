/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Config 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testlib

//go:generate ln -sfn plugin_ent_test.nogo plugin_ent_test.go

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
)

// NewPlugin provides the enterprise plugin.
// This is overwritten by `plugin_ent_test.go` which is a symlink to `plugin_ent_test.nogo`.
// This setup is here so the CI lints pass (they cannot check whether the go mod is tidied
// because they don't have access to the enterprise code.
// This will be simplified once we move to a monorepo and OSS becomes a mirror.
//
// If you have `e` checked out locally, you can run `go generate` to turn on enterprise tests.
var NewPlugin = func(mod modules.Modules) (plugin.Plugin, error) {
	return nil, trace.NotImplemented("No enterprise plugin")
}
