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

import { Link as InternalLink } from 'react-router-dom';

import { CardTile, Flex, H2, P2, Text } from 'design';
import * as Icons from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { SyncStamp } from 'design/SyncStamp/SyncStamp';

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

type Item = 'clusters' | 'databases' | 'instances';

type StatCardProps = {
  name: string;
  item: Item;
  resource: AwsResource;
  summary?: ResourceTypeSummary;
};

export function StatCard({ name, item, resource, summary }: StatCardProps) {
  const updated = summary?.discoverLastSync
    ? new Date(summary?.discoverLastSync)
    : undefined;
  const term = getResourceTerm(resource);

  if (!summary || !foundResource(summary)) {
    return <EnrollCard resource={resource} item={item} />;
  }

  return (
    <CardTile
      width="33%"
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
          <Flex alignItems="center">
            <ResourceIcon name={resource} mr={2} width="32px" height="32px" />
            <H2>{resource.toUpperCase()}</H2>
          </Flex>
          <P2 mb={2}>
            Auto enrolled {resource.toUpperCase()} {item} matching configured
            labels
          </P2>
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
        <SyncStamp date={updated} />
      </Flex>
    </CardTile>
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
