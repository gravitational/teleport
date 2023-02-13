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

package auth

import (
	"github.com/gravitational/teleport-plugins/access/common/auth/oauth"
	"github.com/gravitational/teleport/api/types"
)

// PluginExchangeService implements the exchange of initial credentials
// for hosted plugins.
type PluginExchangeService interface {
	// GetExchanger returns the Exchanger implementation
	// for the given plugin according to its type.
	// If the exchanger can not be found (e.g. exchanger for this type is not configured),
	// nil is returned.
	GetExchanger(types.Plugin) oauth.Exchanger
}
