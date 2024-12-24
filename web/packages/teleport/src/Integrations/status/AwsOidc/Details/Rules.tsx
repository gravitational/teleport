/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { useParams } from 'react-router';

import { Flex } from 'design';
import Table, { LabelCell } from 'design/DataTable';
import { MultiselectMenu } from 'shared/components/Controls/MultiselectMenu';

import { useServerSidePagination } from 'teleport/components/hooks';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import {
  awsRegionMap,
  IntegrationDiscoveryRule,
  IntegrationKind,
  integrationService,
  Regions,
} from 'teleport/services/integrations';

export function Rules() {
  const { name, resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const [regionFilter, setRegionFilter] = useState<string[]>([]);
  const serverSidePagination =
    useServerSidePagination<IntegrationDiscoveryRule>({
      pageSize: 20,
      fetchFunc: async () => {
        // today the rules endpoint requires region. If none are selected, request all.
        const regions =
          regionFilter.length === 0 ? Object.keys(awsRegionMap) : regionFilter;
        const { rules, nextKey } =
          await integrationService.fetchIntegrationRules(
            name,
            resourceKind,
            regions
          );
        return { agents: rules, nextKey };
      },
      clusterId: '',
      params: {},
    });

  useEffect(() => {
    serverSidePagination.fetch();
  }, [regionFilter]);

  return (
    <>
      <MultiselectMenu
        options={Object.keys(awsRegionMap).map(r => ({
          value: r as Regions,
          label: (
            <Flex justifyContent="space-between">
              <div>{awsRegionMap[r]}&nbsp;&nbsp;</div>
              <div>{r}</div>
            </Flex>
          ),
        }))}
        onChange={regions => setRegionFilter(regions)}
        selected={regionFilter}
        label="Region"
        tooltip="Filter by region"
      />
      <Table<IntegrationDiscoveryRule>
        data={serverSidePagination.fetchedData.agents || undefined}
        columns={[
          {
            key: 'region',
            headerText: 'Region',
          },
          {
            key: 'labelMatcher',
            headerText: getResourceTerm(resourceKind),
            render: ({ labelMatcher }) => (
              <LabelCell data={labelMatcher.map(l => `${l.name}:${l.value}`)} />
            ),
          },
        ]}
        emptyText={`No ${resourceKind} rules`}
        fetching={{
          fetchStatus: serverSidePagination.fetchStatus,
          onFetchNext: serverSidePagination.fetchNext,
          onFetchPrev: serverSidePagination.fetchPrev,
        }}
      />
    </>
  );
}

function getResourceTerm(resource: AwsResource): string {
  switch (resource) {
    case AwsResource.rds:
      return 'Tags';
    default:
      return 'Labels';
  }
}
