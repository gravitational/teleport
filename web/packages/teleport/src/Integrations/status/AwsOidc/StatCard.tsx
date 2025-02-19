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

import { formatDistanceStrict } from 'date-fns';
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import { Card, Flex, H2, Text } from 'design';
import * as Icons from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';

import cfg from 'teleport/config';
import { EnrollCard } from 'teleport/Integrations/status/AwsOidc/EnrollCard';
import {
  IntegrationKind,
  ResourceTypeSummary,
} from 'teleport/services/integrations';

export enum AwsResource {
  ec2 = 'ec2',
  eks = 'eks',
  rds = 'rds',
}

type StatCardProps = {
  name: string;
  resource: AwsResource;
  summary?: ResourceTypeSummary;
};

export function StatCard({ name, resource, summary }: StatCardProps) {
  const updated = summary?.discoverLastSync
    ? new Date(summary?.discoverLastSync)
    : undefined;
  const term = getResourceTerm(resource);

  if (!summary || !foundResource(summary)) {
    return <EnrollCard resource={resource} />;
  }

  return (
    <SelectCard
      data-testid={`${resource}-stats`}
      as={InternalLink}
      to={cfg.getIntegrationStatusResourcesRoute(
        IntegrationKind.AwsOidc,
        name,
        resource
      )}
    >
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        minHeight="220px"
      >
        <Flex flexDirection="column" gap={2}>
          <Flex alignItems="center" mb={2}>
            <ResourceIcon name={resource} mr={2} width="32px" height="32px" />
            <H2>{resource.toUpperCase()}</H2>
          </Flex>
          <Flex justifyContent="space-between" data-testid="rules">
            <Text>Enrollment Rules </Text>
            <Text>{summary?.rulesCount || 0}</Text>
          </Flex>
          {resource === AwsResource.rds && (
            <Flex justifyContent="space-between" data-testid="rds-agents">
              <Text>Agents </Text>
              <Text>{summary?.ecsDatabaseServiceCount || 0}</Text>
            </Flex>
          )}
          <Flex justifyContent="space-between" data-testid="enrolled">
            <Text>Enrolled {term} </Text>
            <Text>{summary?.resourcesEnrollmentSuccess || 0}</Text>
          </Flex>
          <Flex justifyContent="space-between" data-testid="failed">
            <Text ml={4}>Failed {term} </Text>
            <Flex gap={1}>
              <Text>{summary?.resourcesEnrollmentFailed || 0}</Text>
              {summary?.resourcesEnrollmentFailed > 0 && (
                <Icons.Warning size="large" color="error.main" />
              )}
            </Flex>
          </Flex>
        </Flex>
        {updated && (
          <Text
            typography="body3"
            color="text.slightlyMuted"
            data-testid="sync"
          >
            Last Sync:{' '}
            {formatDistanceStrict(new Date(updated), new Date(), {
              addSuffix: true,
            })}
          </Text>
        )}
      </Flex>
    </SelectCard>
  );
}

function getResourceTerm(resource: AwsResource): string {
  switch (resource) {
    case AwsResource.rds:
      return 'Databases';
    case AwsResource.eks:
      return 'Clusters';
    case AwsResource.ec2:
    default:
      return 'Instances';
  }
}

function foundResource(resource: ResourceTypeSummary): boolean {
  if (!resource || Object.keys(resource).length === 0) {
    return false;
  }

  if (resource.ecsDatabaseServiceCount != 0) {
    return true;
  }

  return resource.rulesCount != 0 || resource.resourcesFound != 0;
}

export const SelectCard = styled(Card)`
  width: 33%;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: 12px;
  border-radius: ${props => props.theme.radii[2]}px;
  border: ${props => `1px solid ${props.theme.colors.levels.surface}`};
  cursor: pointer;
  text-decoration: none;
  color: ${props => props.theme.colors.text.main};

  &:hover {
    background-color: ${props => props.theme.colors.levels.elevated};
    box-shadow: ${({ theme }) => theme.boxShadow[2]};
  }
`;
