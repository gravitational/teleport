/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { MemoryRouter } from 'react-router';

import { Box, H3 } from 'design';

import { ConsoleCard } from 'teleport/Integrations/status/AwsOidc/Cards/ConsoleCard';
import {
  AwsResource,
  StatCard,
} from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';

export default {
  title: 'Teleport/Integrations/AwsOidc/Cards',
};

export function StatCards() {
  const baseSummary = {
    discoverLastSync: 9,
    ecsDatabaseServiceCount: 0,
    resourcesEnrollmentFailed: 7,
    resourcesEnrollmentSuccess: 8,
    resourcesFound: 6,
    rulesCount: 5,
    unresolvedUserTasks: 10,
  };

  const cases = [
    {
      name: 'Enrolled EC2',
      props: {
        name: 'some-name',
        resource: AwsResource.ec2,
        summary: baseSummary,
      },
    },
    {
      name: 'Un-enrolled EC2',
      props: {
        name: 'some-name',
        resource: AwsResource.ec2,
      },
    },
    {
      name: 'Enrolled EKS',
      props: {
        name: 'some-name',
        resource: AwsResource.eks,
        summary: baseSummary,
      },
    },
    {
      name: 'Un-enrolled EKS',
      props: {
        name: 'some-name',
        resource: AwsResource.eks,
      },
    },
    {
      name: 'Enrolled RDS',
      props: {
        name: 'some-name',
        resource: AwsResource.rds,
        summary: {
          ...baseSummary,
          ecsDatabaseServiceCount: 22,
        },
      },
    },
    {
      name: 'Un-enrolled RDS',
      props: {
        name: 'some-name',
        resource: AwsResource.rds,
      },
    },
  ];
  return (
    <MemoryRouter>
      {cases.map(c => (
        <Box key={c.name} mb={2}>
          <H3>{c.name}</H3>
          <StatCard key={c.name} {...c.props} />
        </Box>
      ))}
    </MemoryRouter>
  );
}

export function ConsoleCards() {
  const cases = [
    {
      name: 'Enrolled CLI',
      props: {
        enrolled: true,
        filters: ['app-*', 'dev-*', 'test'],
        groups: 78,
        lastUpdated: 1754427370000,
        profiles: 5,
        roles: 3,
      },
    },
    {
      name: 'Un-enrolled CLI',
      props: {
        enrolled: false,
      },
    },
  ];
  return (
    <MemoryRouter>
      {cases.map(c => (
        <Box key={c.name} mb={2}>
          <H3>{c.name}</H3>
          <ConsoleCard key={c.name} {...c.props} />
        </Box>
      ))}
    </MemoryRouter>
  );
}
