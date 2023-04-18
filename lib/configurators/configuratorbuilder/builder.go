// Copyright 2022 Gravitational, Inc
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

package configuratorbuilder

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/configurators"
	"github.com/gravitational/teleport/lib/configurators/aws"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// BuildConfigurators reads the configuration and returns a list of
// configurators. Configurators that are "empty" are not returned.
func BuildConfigurators(flags configurators.BootstrapFlags) ([]configurators.Configurator, error) {
	fileConfig, err := config.ReadFromFile(flags.ConfigPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceCfg := servicecfg.MakeDefaultConfig()
	if err := config.ApplyFileConfig(fileConfig, serviceCfg); err != nil {
		return nil, trace.Wrap(err)
	}

	awsConfigurator, err := aws.NewAWSConfigurator(aws.ConfiguratorConfig{
		Flags:         flags,
		ServiceConfig: serviceCfg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var configurators []configurators.Configurator
	if !awsConfigurator.IsEmpty() {
		configurators = append(configurators, awsConfigurator)
	}

	return configurators, nil
}
