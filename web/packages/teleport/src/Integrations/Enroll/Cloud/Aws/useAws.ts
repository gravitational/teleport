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
  IntegrationConfig,
  WildcardRegion,
} from './types';

export function useAws(initialAwsConfig?: Partial<AwsConfig>) {
  const defaultAwsConfig: AwsConfig = {
    integration: {
      name: '',
      roleArn: '',
    },
    accountId: '',
    regions: ['*'] as WildcardRegion,
    ec2Config: {
      enabled: true,
      tags: [],
    },
  };

  const mergedConfig: AwsConfig = {
    ...defaultAwsConfig,
    ...initialAwsConfig,
    integration: {
      ...defaultAwsConfig.integration,
      ...initialAwsConfig?.integration,
    },
    ec2Config: {
      ...defaultAwsConfig.ec2Config,
      ...initialAwsConfig?.ec2Config,
    },
  };

  const [awsConfig, setAwsConfig] = useState<AwsConfig>(mergedConfig);

  const setIntegration = (
    updater: (prev: IntegrationConfig) => IntegrationConfig
  ) => {
    setAwsConfig(prevAwsConfig => ({
      ...prevAwsConfig,
      integration: updater(prevAwsConfig.integration),
    }));
  };

  const setEc2Config = (updater: (prev: Ec2Config) => Ec2Config) => {
    setAwsConfig(prevAwsConfig => ({
      ...prevAwsConfig,
      ec2Config: updater(prevAwsConfig.ec2Config),
    }));
  };

  return {
    awsConfig,
    setAwsConfig,
    setIntegration,
    setEc2Config,
  };
}
