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

import { useEffect, useState } from 'react';
import { useLocation } from 'react-router';

import { Validator } from 'shared/components/Validation';

import { awsRegionMap, Regions } from 'teleport/services/integrations';

type eksConfig = {
  enabled: boolean;
  matchers: AWSLabel[];
  enableAppDiscovery: boolean;
};

type rdsConfig = {
  enabled: boolean;
  matchers: AWSLabel[];
};

type ec2Config = {
  enabled: boolean;
  matchers: AWSLabel[];
};

type integrationConfig = {
  name: string;
  roleName: string;
  roleArn: string;
};

type awsConfig = {
  accountId: string;
  regions: Regions[];
};

type AWSLabel = {
  name: string;
  value: string;
};

export function useCloudAws() {
  const [integrationConfig, setIntegrationConfig] = useState<integrationConfig>(
    {
      name: '',
      roleName: '',
      roleArn: '',
    }
  );

  const [awsConfig, setAwsConfig] = useState<awsConfig>({
    accountId: '',
    regions: [],
  });

  const [eksConfig, setEksConfig] = useState<eksConfig>({
    enabled: false,
    enableAppDiscovery: true,
    matchers: [],
  });

  const [rdsConfig, setRdsConfig] = useState<rdsConfig>({
    enabled: false,
    matchers: [],
  });

  const [ec2Config, setEc2Config] = useState<ec2Config>({
    enabled: false,
    matchers: [],
  });

  return {
    integrationConfig,
    setIntegrationConfig,
    awsConfig,
    setAwsConfig,
    eksConfig,
    setEksConfig,
    rdsConfig,
    setRdsConfig,
    ec2Config,
    setEc2Config,
  };
}
