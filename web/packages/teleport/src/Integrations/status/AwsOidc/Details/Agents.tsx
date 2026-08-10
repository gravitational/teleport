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

import { Box, ButtonPrimary, Indicator } from 'design';
import { Danger } from 'design/Alert';
import Table, { LabelCell } from 'design/DataTable';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import {
  AWSOIDCDeployedDatabaseService,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';

export function Agents() {
  const { name, resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const [servicesAttempt, fetchServices] = useAsync(() => {
    return integrationService.fetchAwsOidcDatabaseServices(name, resourceKind);
  });

  useEffect(() => {
    fetchServices();
  }, []);

  if (servicesAttempt.status === 'processing') {
    return (
      <Box textAlign="center" mt={4}>
        <Indicator />
      </Box>
    );
  }

  return (
    <>
      {servicesAttempt.status === 'error' && (
        <Danger>{servicesAttempt.statusText}</Danger>
      )}
      <Table<AWSOIDCDeployedDatabaseService>
        data={servicesAttempt.data?.services}
        row={{
          onClick: (item: AWSOIDCDeployedDatabaseService) =>
            window.open(item.dashboardUrl, '_blank'),
          getStyle: () => ({
            cursor: 'pointer',
          }),
        }}
        columns={[
          {
            key: 'name',
            headerText: 'Service Name',
          },
          {
            key: 'matchingLabels',
            headerText: 'Labels',
            render: ({ matchingLabels }) => (
              <LabelCell
                data={matchingLabels.map(l => `${l.name}:${l.value}`)}
              />
            ),
          },
        ]}
        emptyText={`No ${resourceKind.toUpperCase()} Agents Found`}
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
        emptyHint="Set up Teleport Discovery service to monitor the dynamic [db] resources registered by the discovery services"
      />
    </>
  );
}
