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

import React, { useState } from 'react';
import { Box, Flex, Input, Text, Toggle } from 'design';
import { FetchStatus } from 'design/DataTable/types';
import { Danger } from 'design/Alert';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { getErrMessage } from 'shared/utils/errorType';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  AwsRdsDatabase,
  RdsEngineIdentifier,
  Regions,
  integrationService,
} from 'teleport/services/integrations';
import { DatabaseEngine } from 'teleport/Discover/SelectResource';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import { Database } from 'teleport/services/databases';
import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';
import { isIamPermError } from 'teleport/Discover/Shared/Aws/error';
import cfg from 'teleport/config';
import {
  DISCOVERY_GROUP_CLOUD,
  DiscoveryConfig,
  createDiscoveryConfig,
} from 'teleport/services/discovery';
import useTeleport from 'teleport/useTeleport';
import { Tabs } from 'teleport/components/Tabs';

import { ActionButtons, Header, Mark, StyledBox } from '../../Shared';

import { useCreateDatabase } from '../CreateDatabase/useCreateDatabase';
import { CreateDatabaseDialog } from '../CreateDatabase/CreateDatabaseDialog';

import { DatabaseList } from './RdsDatabaseList';
import { AutoEnrollDialog } from './AutoEnrollDialog';

type TableData = {
  items: CheckedAwsRdsDatabase[];
  fetchStatus: FetchStatus;
  startKey?: string;
  currRegion?: Regions;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  startKey: '',
};

// CheckedAwsRdsDatabase is a type to describe that a
// AwsRdsDatabase has been checked (by its resource id)
// with the backend whether or not a database server already
// exists for it.
export type CheckedAwsRdsDatabase = AwsRdsDatabase & {
  dbServerExists?: boolean;
};

export function EnrollRdsDatabase() {
  const {
    createdDb,
    pollTimeout,
    registerDatabase,
    attempt: registerAttempt,
    clearAttempt: clearRegisterAttempt,
    nextStep,
    fetchDatabaseServers,
  } = useCreateDatabase();

  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();

  const { agentMeta, resourceSpec, updateAgentMeta, emitErrorEvent } =
    useDiscover();
  const { attempt: fetchDbAttempt, setAttempt: setFetchDbAttempt } =
    useAttempt('');
  const { attempt: autoDiscoverAttempt, setAttempt: setAutoDiscoverAttempt } =
    useAttempt('');

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    startKey: '',
    fetchStatus: 'disabled',
  });
  const [selectedDb, setSelectedDb] = useState<CheckedAwsRdsDatabase>();
  const [wantAutoDiscover, setWantAutoDiscover] = useState(true);
  const [autoDiscoveryCfg, setAutoDiscoveryCfg] = useState<DiscoveryConfig>();
  const [requiredVpcs, setRequiredVpcs] = useState<Record<string, string[]>>();
  const [discoveryGroupName, setDiscoveryGroupName] = useState(() =>
    cfg.isCloud ? '' : 'aws-prod'
  );

  function fetchDatabasesWithNewRegion(region: Regions) {
    // Clear table when fetching with new region.
    fetchDatabases({ ...emptyTableData, currRegion: region });
  }

  function fetchNextPage() {
    fetchDatabases({ ...tableData });
  }

  function refreshDatabaseList() {
    setSelectedDb(null);
    // When refreshing, start the table back at page 1.
    fetchDatabases({ ...tableData, startKey: '', items: [] });
  }

  async function fetchDatabases(data: TableData) {
    const integrationName = agentMeta.awsIntegration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    setFetchDbAttempt({ status: 'processing' });

    try {
      const { databases: fetchedRdsDbs, nextToken } =
        await integrationService.fetchAwsRdsDatabases(
          integrationName,
          getRdsEngineIdentifier(resourceSpec.dbMeta?.engine),
          {
            region: data.currRegion,
            nextToken: data.startKey,
          }
        );

      // Abort if there were no rds dbs for the selected region.
      if (fetchedRdsDbs.length <= 0) {
        setFetchDbAttempt({ status: 'success' });
        setTableData({ ...data, fetchStatus: 'disabled' });
        return;
      }

      // Check if fetched rds databases have a database
      // server for it, to prevent user from enrolling
      // the same db and getting an error from it.

      // Build the predicate string that will query for
      // all the fetched rds dbs by its resource ids.
      const resourceIds: string[] = fetchedRdsDbs.map(
        d => `resource.spec.aws.rds.resource_id == "${d.resourceId}"`
      );
      const query = resourceIds.join(' || ');
      const { agents: fetchedDbServers } = await fetchDatabaseServers(
        query,
        fetchedRdsDbs.length // limit
      );

      const dbServerLookupByResourceId: Record<string, Database> = {};
      fetchedDbServers.forEach(
        d => (dbServerLookupByResourceId[d.aws.rds.resourceId] = d)
      );

      // Check for db server matches.
      const checkedRdsDbs: CheckedAwsRdsDatabase[] = fetchedRdsDbs.map(rds => {
        const dbServer = dbServerLookupByResourceId[rds.resourceId];
        if (dbServer) {
          return {
            ...rds,
            dbServerExists: true,
          };
        }
        return rds;
      });

      setFetchDbAttempt({ status: 'success' });
      setTableData({
        currRegion: data.currRegion,
        startKey: nextToken,
        fetchStatus: nextToken ? '' : 'disabled',
        // concat each page fetch.
        items: [...data.items, ...checkedRdsDbs],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchDbAttempt({ status: 'failed', statusText: errMsg });
      setTableData(data); // fallback to previous data
      emitErrorEvent(`database fetch error: ${errMsg}`);
    }
  }

  function handleAndEmitRequestError(
    err: Error,
    cfg: { errorPrefix?: string; setAttempt?(attempt: Attempt): void }
  ) {
    const message = getErrMessage(err);
    if (cfg.setAttempt) {
      cfg.setAttempt({
        status: 'failed',
        statusText: `${cfg.errorPrefix}${message}`,
      });
    }
    emitErrorEvent(`${cfg.errorPrefix}${message}`);
  }

  async function enableAutoDiscovery() {
    setAutoDiscoverAttempt({ status: 'processing' });

    let requiredVpcsAndSubnets = requiredVpcs;
    if (!requiredVpcsAndSubnets) {
      try {
        const { spec, name: integrationName } = agentMeta.awsIntegration;
        const accountId = spec.roleArn
          .split('arn:aws:iam::')[1]
          .substring(0, 12);
        requiredVpcsAndSubnets =
          await integrationService.fetchAwsRdsRequiredVpcs(integrationName, {
            region: tableData.currRegion,
            accountId,
          });

        setRequiredVpcs(requiredVpcsAndSubnets);
      } catch (err) {
        handleAndEmitRequestError(err, {
          errorPrefix: 'failed to collect vpc ids and its subnets: ',
          setAttempt: setAutoDiscoverAttempt,
        });
        return;
      }
    }

    // Only create a discovery config after successfully fetching
    // required vpcs. This is to avoid creating a unused auto discovery
    // config if user quits in the middle of things not working.
    let discoveryConfig = autoDiscoveryCfg;
    if (!discoveryConfig) {
      try {
        discoveryConfig = await createDiscoveryConfig(clusterId, {
          name: crypto.randomUUID(),
          discoveryGroup: cfg.isCloud
            ? DISCOVERY_GROUP_CLOUD
            : discoveryGroupName,
          aws: [
            {
              types: ['rds'],
              regions: [tableData.currRegion],
              tags: { '*': ['*'] },
              integration: agentMeta.awsIntegration.name,
            },
          ],
        });
        setAutoDiscoveryCfg(discoveryConfig);
      } catch (err) {
        handleAndEmitRequestError(err, {
          errorPrefix: 'failed to create discovery config: ',
          setAttempt: setAutoDiscoverAttempt,
        });
        return;
      }
    }

    setAutoDiscoverAttempt({ status: 'success' });
    updateAgentMeta({
      ...(agentMeta as DbMeta),
      autoDiscovery: {
        config: discoveryConfig,
        requiredVpcsAndSubnets,
      },
      serviceDeployedMethod:
        Object.keys(requiredVpcsAndSubnets).length > 0 ? undefined : 'skipped',
      awsRegion: tableData.currRegion,
    });
  }

  function clear() {
    clearRegisterAttempt();

    if (fetchDbAttempt.status === 'failed') {
      setFetchDbAttempt({ status: '' });
    }
    if (tableData.items.length > 0) {
      setTableData(emptyTableData);
    }
    if (selectedDb) {
      setSelectedDb(null);
    }
  }

  function handleOnProceed() {
    if (wantAutoDiscover) {
      enableAutoDiscovery();
    } else {
      const isNewDb = selectedDb.name !== createdDb?.name;
      registerDatabase(
        {
          name: selectedDb.name,
          protocol: selectedDb.engine,
          uri: selectedDb.uri,
          labels: selectedDb.labels,
          awsRds: selectedDb,
          awsRegion: tableData.currRegion,
        },
        // Corner case where if registering db fails a user can:
        //   1) change region, which will list new databases or
        //   2) select a different database before re-trying.
        isNewDb
      );
    }
  }

  let DialogComponent;
  if (registerAttempt.status !== '') {
    DialogComponent = (
      <CreateDatabaseDialog
        pollTimeout={pollTimeout}
        attempt={registerAttempt}
        next={nextStep}
        close={clearRegisterAttempt}
        retry={handleOnProceed}
        dbName={selectedDb.name}
      />
    );
  } else if (autoDiscoverAttempt.status !== '') {
    DialogComponent = (
      <AutoEnrollDialog
        attempt={autoDiscoverAttempt}
        next={nextStep}
        close={() => setAutoDiscoverAttempt({ status: '' })}
        retry={handleOnProceed}
        region={tableData.currRegion}
        skipDeployment={requiredVpcs && Object.keys(requiredVpcs).length === 0}
      />
    );
  }

  const hasIamPermError = isIamPermError(fetchDbAttempt);
  const showContent = !hasIamPermError && tableData.currRegion;
  const showAutoEnrollToggle = fetchDbAttempt.status === 'success';

  // (Temp)
  // Self hosted auto enroll is different from cloud.
  // For cloud, we already run the discovery service for customer.
  // For on-prem, user has to run their own discovery service.
  // We hide the RDS table for on-prem if they are wanting auto discover
  // because it takes up so much space to give them instructions.
  // Future work will simply provide user a script so we can show the table then.
  const showTable = cfg.isCloud || !wantAutoDiscover;

  return (
    <Box maxWidth="800px">
      <Header>Enroll a RDS Database</Header>
      {fetchDbAttempt.status === 'failed' && !hasIamPermError && (
        <Danger mt={3}>{fetchDbAttempt.statusText}</Danger>
      )}
      <Text mt={4}>
        Select the AWS Region you would like to see databases for:
      </Text>
      <AwsRegionSelector
        onFetch={fetchDatabasesWithNewRegion}
        onRefresh={refreshDatabaseList}
        clear={clear}
        disableSelector={fetchDbAttempt.status === 'processing'}
      />
      {showContent && (
        <>
          {showAutoEnrollToggle && (
            <ToggleSection
              wantAutoDiscover={wantAutoDiscover}
              toggleWantAutoDiscover={() => setWantAutoDiscover(b => !b)}
              isDisabled={tableData.items.length === 0}
              discoveryGroupName={discoveryGroupName}
              setDiscoveryGroupName={setDiscoveryGroupName}
              clusterPublicUrl={ctx.storeUser.state.cluster.publicURL}
            />
          )}
          {showTable && (
            <DatabaseList
              wantAutoDiscover={wantAutoDiscover}
              items={tableData.items}
              fetchStatus={tableData.fetchStatus}
              selectedDatabase={selectedDb}
              onSelectDatabase={setSelectedDb}
              fetchNextPage={fetchNextPage}
            />
          )}
        </>
      )}
      {hasIamPermError && (
        <Box mb={5}>
          <ConfigureIamPerms
            kind="rds"
            region={tableData.currRegion}
            integrationRoleArn={agentMeta.awsIntegration.spec.roleArn}
          />
        </Box>
      )}
      {showContent && showAutoEnrollToggle && wantAutoDiscover && (
        <Text mt={4} mb={-3}>
          <b>Note:</b> Auto-enroll will enroll <Mark>all</Mark> database engines
          in this region (e.g. PostgreSQL, MySQL, Aurora).
        </Text>
      )}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={
          fetchDbAttempt.status === 'processing' ||
          (!wantAutoDiscover && !selectedDb) ||
          hasIamPermError ||
          fetchDbAttempt.status === 'failed' ||
          (!cfg.isCloud && !discoveryGroupName)
        }
      />
      {DialogComponent}
    </Box>
  );
}

function getRdsEngineIdentifier(engine: DatabaseEngine): RdsEngineIdentifier {
  switch (engine) {
    case DatabaseEngine.MySql:
      return 'mysql';
    case DatabaseEngine.Postgres:
      return 'postgres';
    case DatabaseEngine.AuroraMysql:
      return 'aurora-mysql';
    case DatabaseEngine.AuroraPostgres:
      return 'aurora-postgres';
  }
}

const discoveryGroupToolTip = `Discovery group name is used to group discovered resources into different sets. \
This parameter is used to prevent Discovery Agents watching different sets of cloud resources from \
colliding against each other and deleting resources created by another services.`;

const discoveryServiceToolTip = `The Discovery Service, is responsible for watching your \
cloud provider and checking if there are any new databases or if there have been any \
modifications to previously discovered databases.`;

function ToggleSection({
  wantAutoDiscover,
  toggleWantAutoDiscover,
  isDisabled,
  discoveryGroupName,
  setDiscoveryGroupName,
  clusterPublicUrl,
}: {
  wantAutoDiscover: boolean;
  isDisabled: boolean;
  toggleWantAutoDiscover(): void;
  discoveryGroupName: string;
  setDiscoveryGroupName(n: string): void;
  clusterPublicUrl: string;
}) {
  return (
    <Box mb={2}>
      <Toggle
        isToggled={wantAutoDiscover}
        onToggle={toggleWantAutoDiscover}
        disabled={isDisabled}
      >
        <Box ml={2} mr={1}>
          Auto-enroll all databases for selected region
        </Box>
        <ToolTipInfo>
          Auto-enroll will automatically identify all RDS databases from the
          selected region and register them as database resources in your
          infrastructure.
        </ToolTipInfo>
      </Toggle>
      {!cfg.isCloud && wantAutoDiscover && (
        <SelfHostedAutoDiscoverDirections
          clusterPublicUrl={clusterPublicUrl}
          discoveryGroupName={discoveryGroupName}
          setDiscoveryGroupName={setDiscoveryGroupName}
        />
      )}
    </Box>
  );
}

const SelfHostedAutoDiscoverDirections = ({
  clusterPublicUrl,
  discoveryGroupName,
  setDiscoveryGroupName,
}: {
  clusterPublicUrl: string;
  discoveryGroupName: string;
  setDiscoveryGroupName(n: string): void;
}) => {
  const yamlContent = `version: v3
teleport:
  join_params:
    token_name: "<YOUR_JOIN_TOKEN_FROM_STEP_1>"
    method: token
  proxy_server: "${clusterPublicUrl}"
auth_service:
  enabled: off
proxy_service:
  enabled: off
ssh_service:
  enabled: off
discovery_service:
  enabled: "yes"
  discovery_group: "${discoveryGroupName}"`;

  return (
    <Box mt={2}>
      <Flex alignItems="center">
        <Text>
          Auto-enrolling requires you to configure a{' '}
          <Mark>Discovery Service</Mark>
        </Text>
        <ToolTipInfo children={discoveryServiceToolTip} />
      </Flex>
      <br />
      <StyledBox mb={5}>
        <Text bold>Step 1: Create a Join Token</Text>
        <Text mb={2}>
          Run the following command against your Teleport Auth Service and save
          it in <Mark>/tmp/token</Mark> on the host that will run the Discovery
          Service.
        </Text>
        <TextSelectCopyMulti
          lines={[
            {
              text: `tctl tokens add --type=discovery`,
            },
          ]}
        />
      </StyledBox>
      <StyledBox mb={5}>
        <Flex alignItems="center">
          <Text bold mr={1}>
            Step 2: Define a Discovery Group name{' '}
          </Text>
          <ToolTipInfo children={discoveryGroupToolTip} />
        </Flex>
        <Box mt={3} width="260px">
          <Input
            value={discoveryGroupName}
            onChange={e => setDiscoveryGroupName(e.target.value)}
            hasError={discoveryGroupName.length == 0}
          />
        </Box>
      </StyledBox>
      <StyledBox mb={5}>
        <Text bold mr={1}>
          Step 3: Create a teleport.yaml file
        </Text>
        <Text mb={2}>
          Use this template to create a <Mark>teleport.yaml</Mark> on the host
          that will run the Discovery Service.
        </Text>
        <TextSelectCopyMulti lines={[{ text: yamlContent }]} bash={false} />
      </StyledBox>
      <StyledBox mb={5}>
        <Text bold mr={1}>
          Step 4: Start Discovery Service
        </Text>
        <Text mb={2}>
          Configure the Discovery Service to start automatically when the host
          boots up by creating a systemd service for it. The instructions depend
          on how you installed the Discovery Service.
        </Text>
        <Tabs
          tabs={[
            {
              title: 'Package Manager',
              content: (
                <Box px={2} pb={2}>
                  <Text mb={2}>
                    On the host where you will run the Discovery Service, enable
                    and start Teleport:
                  </Text>
                  <TextSelectCopyMulti
                    lines={[
                      {
                        text: `sudo systemctl enable teleport`,
                      },
                      {
                        text: `sudo systemctl start teleport`,
                      },
                    ]}
                  />
                </Box>
              ),
            },
            {
              title: `TAR Archive`,
              content: (
                <Box px={2} pb={2}>
                  <Text mb={2}>
                    On the host where you will run the Discovery Service, create
                    a systemd service configuration for Teleport, enable the
                    Teleport service, and start Teleport:
                  </Text>
                  <TextSelectCopyMulti
                    lines={[
                      {
                        text: `sudo teleport install systemd -o /etc/systemd/system/teleport.service`,
                      },
                      {
                        text: `sudo systemctl enable teleport`,
                      },
                      {
                        text: `sudo systemctl start teleport`,
                      },
                    ]}
                  />
                </Box>
              ),
            },
          ]}
        />
        <Text mt={2}>
          You can check the status of the Discovery Service with{' '}
          <Mark>systemctl status teleport</Mark> and view its logs with{' '}
          <Mark>journalctl -fu teleport</Mark>.
        </Text>
      </StyledBox>
    </Box>
  );
};
