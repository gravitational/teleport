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

import { useEffect } from 'react';
import { useParams } from 'react-router';
import { Link as InternalLink } from 'react-router-dom';

import { ButtonPrimary } from 'design';
import Table, { LabelCell } from 'design/DataTable';

import { useServerSidePagination } from 'teleport/components/hooks';
import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import {
  IntegrationDiscoveryRule,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';

export function Rules() {
  const { name, resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const serverSidePagination =
    useServerSidePagination<IntegrationDiscoveryRule>({
      pageSize: 20,
      fetchFunc: async () => {
        const { rules, nextKey } =
          await integrationService.fetchIntegrationRules(name, resourceKind);
        return { agents: rules, nextKey };
      },
      clusterId: '',
      params: {},
    });

  useEffect(() => {
    serverSidePagination.fetch();
  }, []);

  return (
    <Table<IntegrationDiscoveryRule>
      data={serverSidePagination?.fetchedData?.agents}
      columns={[
        {
          key: 'region',
          headerText: 'Region',
        },
        {
          key: 'labelMatcher',
          headerText: 'Labels',
          render: ({ labelMatcher }) => (
            <LabelCell data={labelMatcher.map(l => `${l.name}:${l.value}`)} />
          ),
        },
      ]}
      emptyText={`No ${resourceKind.toUpperCase()} Rules Found`}
      emptyHint={
        resourceKind === AwsResource.rds &&
        'Discover AWS-hosted databases automatically and register them with your Teleport cluster'
      }
      emptyButton={
        <ButtonPrimary
          as={InternalLink}
          to={{
            pathname: cfg.routes.discover,
            state: { searchKeywords: resourceKind },
          }}
        >
          Add Enrollment Rule
        </ButtonPrimary>
      }
      pagination={{ pageSize: serverSidePagination.pageSize }}
      fetching={{
        fetchStatus: serverSidePagination.fetchStatus,
        onFetchNext: serverSidePagination.fetchNext,
        onFetchPrev: serverSidePagination.fetchPrev,
      }}
    />
  );
}
