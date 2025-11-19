/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useState } from 'react';

import {
  AwsConfig,
  Ec2Config,
  EksConfig,
  IntegrationConfig,
  RdsConfig,
  WildcardRegion,
} from './types';

export function useAws() {
  const [awsConfig, setAwsConfig] = useState<AwsConfig>({
    integration: {
      name: '',
      roleArn: '',
    },
    accountId: '',
    regions: ['*'] as WildcardRegion,
  });

  const [ec2Config, setEc2Config] = useState<Ec2Config>({
    enabled: false,
    tags: [],
  });

  const [rdsConfig, setRdsConfig] = useState<RdsConfig>({
    enabled: false,
    tags: [],
  });

  const [eksConfig, setEksConfig] = useState<EksConfig>({
    enabled: false,
    tags: [],
    enableAppDiscovery: false,
  });

  const setIntegration = (
    updater: (prev: IntegrationConfig) => IntegrationConfig
  ) => {
    setAwsConfig(prevAwsConfig => ({
      ...prevAwsConfig,
      integration: updater(prevAwsConfig.integration),
    }));
  };

  return {
    awsConfig,
    setAwsConfig,
    integration: awsConfig.integration,
    setIntegration,
    ec2Config,
    rdsConfig,
    eksConfig,
    setEc2Config,
    setRdsConfig,
    setEksConfig,
  };
}
