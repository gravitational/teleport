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
import { useHistory, Link, useParams } from 'react-router-dom';
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
import { AwsResourceKind } from '../../Shared';
import { ListEc2Instances } from '../Ec2/ListEc2Instances';
import { ListEksClusters } from '../Eks/ListEksClusters';
import { ListRdsDatabases } from '../Rds/ListRdsDatabases';

export function AwsOidcResources() {
  const {
    type: integrationType,
    name: integrationName,
    resourceKind,
  } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind?: AwsResourceKind;
  }>();

  const ctx = useTeleport();
  const integrationAccess = ctx.storeUser.getIntegrationsAccess();
  const hasIntegrationReadAccess = integrationAccess.read;

  const [attempt, fetchIntegration] = useAsync(async () => {
    // First check if integration is found before doing other api calls.
    // A user can land on this route, without going through the integration list -> view status flow
    const integration =
      await integrationService.fetchIntegration(integrationName);

    console.log('---- fetched integration: ', integration);

    return integration;
  });

  useEffect(() => {
    if (hasIntegrationReadAccess) {
      fetchIntegration();
    }
  }, []);

  let List;

  console.log('--- here: ', resourceKind);
  if (resourceKind === 'ec2') {
    console.log('-- here eljrwlekjrwlek jlskfjsl');
    List = <ListEc2Instances />;
  } else if (resourceKind == 'eks') {
    List = <ListEksClusters />;
  } else if (resourceKind === 'rds') {
    List = <ListRdsDatabases />;
  } else {
    return <>resource kind "{resourceKind}" not supported</>;
  }

  return (
    <FeatureBox>
      <SpaceBetweenFlexedHeader>
        <AwsOidcHeader
          integration={attempt.data}
          attemptStatus={attempt.status}
          secondaryHeader={resourceKind}
        />
      </SpaceBetweenFlexedHeader>
      {(attempt.status === 'processing' || attempt.status === '') && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'error' && <Alert children={attempt.statusText} />}
      {attempt.status === 'success' && <>{List}</>}
    </FeatureBox>
  );
}
