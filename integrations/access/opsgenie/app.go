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

package opsgenie

import (
	"github.com/gravitational/teleport/integrations/access/common"
)

const (
	// opsgeniePluginName is used to tag Opsgenie GenericPluginData and as a Delegator in Audit log.
	opsgeniePluginName = "opsgenie"
)

// NewOpsgenieApp initializes a new teleport-opsgenie app and returns it.
func NewOpsgenieApp(conf *Config) *common.BaseApp {
	return common.NewApp(conf, opsgeniePluginName)
}
