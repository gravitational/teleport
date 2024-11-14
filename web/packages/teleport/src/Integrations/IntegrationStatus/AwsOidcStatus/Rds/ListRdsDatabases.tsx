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
import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';
import {
  Flex,
  H1,
  H2,
  H3,
  H4,
  H5,
  H6,
  Text,
  Box,
  Alert,
  Indicator,
} from 'design';
import { SyncAlt } from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';
import * as Icons from 'design/Icon';
import Table, { LabelCell } from 'design/DataTable';

import { Integration } from 'teleport/services/integrations';
import { useTeleport } from 'teleport/index';

import {
  Panel,
  PanelTitle,
  CenteredSpaceBetweenFlex,
  CustomLabel,
  ErrorTooltip,
  InnerCard,
  GappedColumnFlex,
  PanelHeader,
  PanelLastSynced,
} from '../../Shared';

import { PanelIcon } from '../../getResourceIcon';
import { Label } from 'teleport/types';
import { ResourceTab } from 'shared/components/UnifiedResources/ResourceTab';

type ResourcesTab = 'discovery_config' | 'agents';
const tabs: { value: ResourcesTab; label: string }[] = [
  { value: 'discovery_config', label: 'Discovery Configs' },
  { value: 'agents', label: 'Agents' }, // TODO is it agents? or services?
];

export function ListRdsDatabases() {
  const ctx = useTeleport();
  // check access to list agents

  const [currTab, setCurrTab] = useState<ResourcesTab>('discovery_config');

  const [
    fetchServicesAttempt,
    fetchRdsDatabaseServices,
    setFetchServicesAttempt,
  ] = useAsync(async () => {
    // TODO
    return Promise.reject(new Error('uh oh error'));
    return Promise.resolve([
      {
        region: 'us-east-1',
        labels: [
          { name: 'env', value: 'staging' },
          { name: 'season', value: 'summer' },
        ],
      },
      {
        region: 'us-east-2',
        labels: [
          { name: 'env', value: 'prod' },
          { name: 'season', value: 'summer' },
          { name: 'drink', value: 'coffee' },
        ],
      },
    ]);
  });

  const [
    fetchConfigsAttempt,
    fetchRdsDiscoveryConfigs,
    setFetchConfigsAttempt,
  ] = useAsync(async () => {
    // TODO
    return Promise.resolve([
      {
        name: 'config-1',
      },
      {
        name: 'config-2',
      },
    ]);
  });

  useEffect(() => {
    // Clear all attempts between tab switches.
    setFetchConfigsAttempt(makeEmptyAttempt());
    setFetchServicesAttempt(makeEmptyAttempt());

    // TODO
    // has access, fetch, else show error
    if (currTab === 'agents') {
      fetchRdsDatabaseServices();
      return;
    }
    if (currTab === 'discovery_config') {
      fetchRdsDiscoveryConfigs();
      return;
    }
  }, [currTab]);

  const isProcessing =
    fetchServicesAttempt.status === 'processing' ||
    fetchConfigsAttempt.status === 'processing';

  const hasError =
    fetchServicesAttempt.status === 'error' ||
    fetchConfigsAttempt.status === 'error';

  return (
    <>
      <Flex mb={3} gap={3}>
        {tabs.map(s => (
          <ResourceTab
            onClick={() => setCurrTab(s.value)}
            disabled={false}
            isSelected={s.value === currTab}
            key={s.value}
            title={s.label}
          />
        ))}
      </Flex>
      {isProcessing && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {/* TODO needs a retry button in the alert banner in case fetch failed */}
      {hasError && (
        <Alert
          children={
            fetchServicesAttempt.statusText || fetchConfigsAttempt.statusText
          }
        />
      )}
      {fetchConfigsAttempt.status === 'success' && (
        <Table
          data={fetchConfigsAttempt.data}
          columns={[{ key: 'name', headerText: 'Name' }]}
          emptyText="No Discovery Configs found"
          isSearchable
        />
      )}
      {fetchServicesAttempt.status === 'success' && (
        <Table
          data={fetchServicesAttempt.data}
          columns={[
            { key: 'region', headerText: 'Region' },
            {
              key: 'labels',
              headerText: 'Labels',
              render: ({ labels }) => (
                <LabelCell data={labels.map(l => `${l.name}: ${l.value}`)} />
              ),
            },
          ]}
          emptyText="No RDS Databases found"
          isSearchable
        />
      )}
    </>
  );
}
