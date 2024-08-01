/**
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

import React, { useState, useEffect } from 'react';
import { Box, Text, Link as ExternalLink, Flex, ButtonSecondary } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';
import Alert, { OutlineInfo } from 'design/Alert/Alert';

import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  Regions,
  Vpc,
  integrationService,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';
import {
  DISCOVERY_GROUP_CLOUD,
  createDiscoveryConfig,
} from 'teleport/services/discovery';
import useTeleport from 'teleport/useTeleport';
import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';
import { CreatedDiscoveryConfigDialog } from 'teleport/Discover/Shared/ConfigureDiscoveryService';

import { ActionButtons } from '../../Shared';

import { DatabaseList } from './RdsDatabaseList';

type TableData = {
  items: AwsRdsDatabase[];
  fetchStatus: FetchStatus;
  instancesStartKey?: string;
  clustersStartKey?: string;
  oneOfError?: string;
};

const emptyTableData = (): TableData => ({
  items: [],
  fetchStatus: 'disabled',
  instancesStartKey: '',
  clustersStartKey: '',
  oneOfError: '',
});

export function AutoEnrollment({
  region,
  vpc,
  disableBtns,
  onFetchAttempt,
  fetchAttempt,
}: {
  region: Regions;
  vpc?: Vpc;
  disableBtns: boolean;
  fetchAttempt: Attempt;
  onFetchAttempt(a: Attempt): void;
  /**
   * key is expected to be set to the ID of the VPC.
   */
  key: string;
}) {
  const hasDatabaseServiceForVpc = !!vpc?.ecsServiceDashboardURL;

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();

  const { agentMeta, updateAgentMeta, emitErrorEvent, nextStep, emitEvent } =
    useDiscover();
  const {
    attempt: createDiscoveryConfigAttempt,
    setAttempt: setCreateDiscoveryConfigAttempt,
  } = useAttempt('');

  const [tableData, setTableData] = useState<TableData>();

  useEffect(() => {
    if (hasDatabaseServiceForVpc) {
      // No need to fetch rds's since in place of rds table
      // we will render a info banner that a db service
      // already exists.
      return;
    }

    if (vpc) {
      // Start with empty table data for new vpc's.
      fetchRdsDatabases(emptyTableData(), vpc);
    }
  }, [vpc]);

  function fetchNextPage() {
    fetchRdsDatabases({ ...tableData }, vpc);
  }

  async function fetchRdsDatabases(data: TableData, vpc: Vpc) {
    const integrationName = agentMeta.awsIntegration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    onFetchAttempt({ status: 'processing' });

    try {
      const {
        databases: fetchedDbs,
        instancesNextToken,
        clustersNextToken,
        oneOfError,
      } = await integrationService.fetchAllAwsRdsEnginesDatabases(
        integrationName,
        {
          region: region,
          instancesNextToken: data.instancesStartKey,
          clustersNextToken: data.clustersStartKey,
          vpcId: vpc.id,
        }
      );

      // Abort if there were no rds dbs for the selected region.
      if (fetchedDbs.length <= 0) {
        onFetchAttempt({ status: 'success' });
        setTableData({ ...data, fetchStatus: 'disabled' });
        return;
      }

      onFetchAttempt({ status: 'success' });
      setTableData({
        instancesStartKey: instancesNextToken,
        clustersStartKey: clustersNextToken,
        fetchStatus: instancesNextToken || clustersNextToken ? '' : 'disabled',
        oneOfError,
        // concat each page fetch.
        items: [...data.items, ...fetchedDbs],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      onFetchAttempt({ status: 'failed', statusText: errMsg });
      setTableData(data); // fallback to previous data
      emitErrorEvent(`database fetch error: ${errMsg}`);
    }
  }

  async function handleOnProceed() {
    // For self-hosted, discovery config needs to be created
    // on the next step since self-hosted needs to manually
    // install a discovery service.
    if (!cfg.isCloud) {
      updateAgentMeta({
        ...(agentMeta as DbMeta),
        awsVpcId: vpc.id,
        awsRegion: region,
        autoDiscovery: {},
      });
      nextStep();
      return;
    }

    try {
      setCreateDiscoveryConfigAttempt({ status: 'processing' });
      // Cloud has a discovery service automatically running so
      // we have everything we need to create a
      const discoveryConfig = await createDiscoveryConfig(clusterId, {
        name: crypto.randomUUID(),
        discoveryGroup: DISCOVERY_GROUP_CLOUD,
        aws: [
          {
            types: ['rds'],
            regions: [region],
            tags: { 'vpc-id': [vpc.id] },
            integration: agentMeta.awsIntegration.name,
          },
        ],
      });

      emitEvent(
        { stepStatus: DiscoverEventStatus.Success },
        {
          eventName: DiscoverEvent.CreateDiscoveryConfig,
        }
      );

      setCreateDiscoveryConfigAttempt({ status: 'success' });
      updateAgentMeta({
        ...(agentMeta as DbMeta),
        autoDiscovery: {
          config: discoveryConfig,
        },
        awsVpcId: vpc.id,
        awsRegion: region,
      });
    } catch (err) {
      const message = getErrMessage(err);
      setCreateDiscoveryConfigAttempt({
        status: 'failed',
        statusText: `failed to create discovery config: ${message}`,
      });
      emitErrorEvent(`failed to create discovery config: ${message}`);
      return;
    }
  }

  const selectedVpc = !!vpc;
  const showTable =
    selectedVpc &&
    !hasDatabaseServiceForVpc &&
    fetchAttempt.status !== 'failed';

  return (
    <>
      {hasDatabaseServiceForVpc && (
        <OutlineInfo mt={3} width="480px" linkColor="buttons.link.default">
          <Box>
            There is a database service already deployed for the selected VPC,
            visit its{' '}
            <ExternalLink target="_blank" href={vpc.ecsServiceDashboardURL}>
              dashboard
            </ExternalLink>{' '}
            to check it out.
          </Box>
        </OutlineInfo>
      )}
      {showTable && (
        <>
          {tableData?.oneOfError && (
            <Alert>
              <Flex alignItems="center" gap={2}>
                {tableData.oneOfError}
                <ButtonSecondary
                  onClick={() => fetchRdsDatabases(emptyTableData(), vpc)}
                >
                  Retry
                </ButtonSecondary>
              </Flex>
            </Alert>
          )}
          <Text mt={3}>List of databases that will be auto enrolled:</Text>
          <DatabaseList
            wantAutoDiscover={true}
            items={tableData?.items || []}
            fetchStatus={tableData?.fetchStatus || 'loading'}
            fetchNextPage={fetchNextPage}
          />
        </>
      )}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={disableBtns || !showTable || hasDatabaseServiceForVpc}
      />
      {createDiscoveryConfigAttempt.status !== '' && (
        <CreatedDiscoveryConfigDialog
          attempt={createDiscoveryConfigAttempt}
          next={nextStep}
          close={() => setCreateDiscoveryConfigAttempt({ status: '' })}
          retry={handleOnProceed}
          region={region}
          notifyAboutDelay={false} // TODO always notify?
        />
      )}
    </>
  );
}
