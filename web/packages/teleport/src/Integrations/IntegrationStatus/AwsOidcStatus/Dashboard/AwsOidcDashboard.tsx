/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useEffect, useState } from 'react';
import { useHistory, Link as InternalLink, useParams } from 'react-router-dom';

import { Indicator, Box, Alert, Flex } from 'design';
import { useAsync } from 'shared/hooks/useAsync';

import { FeatureBox } from 'teleport/components/Layout';
import { Integration, IntegrationKind } from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';
import {
  integrationService,
  makeIntegration,
} from 'teleport/services/integrations/integrations';
import cfg from 'teleport/config';

import { AwsOidcHeader, SpaceBetweenFlexedHeader } from '../AwsOidcHeader';
import { PanelEc2Stats } from '../Ec2/PanelEc2Stats';
import { PanelRdsStats } from '../Rds/PanelRdsStats';
import { PanelEksStats } from '../Eks/PanelEksStats';
import { useAwsOidcStatus } from '../useAwsOidcStatus';

import { AwsResourceKind } from '../../Shared';

import { AwsOidcSettings } from './AwsOidcSettings';

export function AwsOidcDashboard() {
  const { attempt } = useAwsOidcStatus();
  const { type: integrationType, name: integrationName } = useParams<{
    type: IntegrationKind;
    name: string;
  }>();

  function getResourcesRoute(kind: AwsResourceKind) {
    return cfg.getIntegrationStatusResourcesRoute(
      integrationType,
      integrationName,
      kind
    );
  }

  return (
    <FeatureBox>
      <SpaceBetweenFlexedHeader>
        <AwsOidcHeader
          integration={attempt.data}
          attemptStatus={attempt.status}
        />
        {attempt.status === 'success' && (
          <AwsOidcSettings integration={attempt.data} />
        )}
      </SpaceBetweenFlexedHeader>
      {(attempt.status === 'processing' || attempt.status === '') && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'error' && <Alert children={attempt.statusText} />}
      {attempt.status === 'success' && (
        <Flex gap={3}>
          <PanelEc2Stats
            integration={attempt.data}
            route={getResourcesRoute('ec2')}
          />
          <PanelRdsStats
            integration={attempt.data}
            route={getResourcesRoute('rds')}
          />
          <PanelEksStats
            integration={attempt.data}
            route={getResourcesRoute('eks')}
          />
        </Flex>
      )}
    </FeatureBox>
  );
}
