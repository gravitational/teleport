/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
