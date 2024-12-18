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

import Table, { LabelCell } from 'design/DataTable';

import { useParams } from 'react-router';

import {
  IntegrationDiscoveryRule,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import { SearchPanel } from 'shared/components/Search';
import { useServerSidePagination } from 'teleport/components/hooks';
import { SortType } from 'design/DataTable/types';

export function Rules() {
  const { name, resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<SortType>({
    fieldName: 'region',
    dir: 'ASC',
  });
  const serverSidePagination =
    useServerSidePagination<IntegrationDiscoveryRule>({
      pageSize: 20,
      fetchFunc: async (_, params) => {
        const { rules, nextKey } =
          await integrationService.fetchIntegrationRules(
            name,
            resourceKind,
            params
          );
        return { agents: rules, nextKey };
      },
      clusterId: '',
      params: { search, sort },
    });

  useEffect(() => {
    serverSidePagination.fetch();
  }, [search, sort]);

  return (
    <Table<IntegrationDiscoveryRule>
      data={serverSidePagination.fetchedData.agents || undefined}
      columns={[
        {
          key: 'region',
          headerText: 'Region',
          isSortable: true,
        },
        {
          key: 'labelMatcher',
          headerText: getResourceTerm(resourceKind),
          isSortable: true,
          onSort: (a, b) => {
            const aStr = a.labelMatcher.toString();
            const bStr = b.labelMatcher.toString();

            if (aStr < bStr) {
              return -1;
            }
            if (aStr > bStr) {
              return 1;
            }

            return 0;
          },
          render: ({ labelMatcher }) => (
            <LabelCell data={labelMatcher.map(l => `${l.name}:${l.value}`)} />
          ),
        },
      ]}
      emptyText={`No ${resourceKind} data`}
      isSearchable
      fetching={{
        fetchStatus: serverSidePagination.fetchStatus,
        onFetchNext: serverSidePagination.fetchNext,
        onFetchPrev: serverSidePagination.fetchPrev,
      }}
      serversideProps={{
        sort: sort,
        setSort: setSort,
        serversideSearchPanel: (
          <SearchPanel
            updateSearch={setSearch}
            updateQuery={null}
            hideAdvancedSearch={true}
            filter={{ search }}
            disableSearch={serverSidePagination.attempt.status === 'processing'}
          />
        ),
      }}
    />
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
