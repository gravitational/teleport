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

import React, { useEffect } from 'react';

import Table, { LabelCell } from 'design/DataTable';

import { useParams } from 'react-router';

import { useAsync } from 'shared/hooks/useAsync';
import { Indicator } from 'design';
import { Danger } from 'design/Alert';

import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';

export function Rules() {
  const { name, resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const [attempt, fetchRules] = useAsync(() =>
    integrationService.fetchIntegrationRules(name, resourceKind)
  );

  useEffect(() => {
    fetchRules();
  }, []);

  if (attempt.status == 'processing') {
    return <Indicator />;
  }

  if (attempt.status == 'error') {
    return <Danger>{attempt.statusText}</Danger>;
  }

  if (!attempt.data) {
    return null;
  }

  return (
    <Table
      data={attempt.data.rules}
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
