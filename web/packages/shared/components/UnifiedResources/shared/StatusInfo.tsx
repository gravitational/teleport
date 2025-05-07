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

import styled, { css } from 'styled-components';

import { Box, ButtonBorder, Indicator, Mark, Text } from 'design';
import { Alert } from 'design/Alert';
import Flex from 'design/Flex';
import {
  BookOpenText,
  Database as DatabaseIcon,
  Warning as WarningIcon,
} from 'design/Icon';
import { ResourceIcon } from 'design/ResourceIcon';
import { H2, H3 } from 'design/Text';
import {
  InfoParagraph,
  InfoTitle,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { useInfiniteScroll } from 'shared/hooks';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { pluralize } from 'shared/utils/text';

import { SharedResourceServer, UnifiedResourceDefinition } from '../types';
import { SingleLineBox } from './SingleLineBox';
import { getDatabaseIconName } from './viewItemsFactory';

export function UnhealthyStatusInfo({
  resource,
  fetch,
  attempt,
  servers = [],
}: {
  resource: UnifiedResourceDefinition;
  fetch(options?: { force?: boolean }): Promise<void>;
  attempt: Attempt;
  servers: SharedResourceServer[];
}) {
  const { setTrigger } = useInfiniteScroll({
    fetch: fetch,
  });

  function retryAttempt() {
    fetch({ force: true });
  }

  const unhealthyOrUnknownServers = servers.filter(
    s => s.targetHealth?.status !== 'healthy'
  );

  return (
    <>
      <Box>
        <ConnectionHeader resource={resource} />
        <InfoParagraph>
          <StatusDescription
            resource={resource}
            numUnhealthyServers={unhealthyOrUnknownServers.length}
          />
        </InfoParagraph>
        <InfoParagraph mb={4}>
          <ButtonBorder
            as="a"
            size="large"
            gap={2}
            intent="primary"
            target="_blank"
            href={getTroubleShootingLink(resource)}
          >
            <BookOpenText /> Troubleshooting Guide
          </ButtonBorder>
        </InfoParagraph>

        <InfoTitle mt={6}>
          Affected Teleport {getAffectedResourceKind(resource)}:
        </InfoTitle>
        {attempt.status === 'failed' && (
          <Alert
            kind="danger"
            primaryAction={{ content: 'Retry', onClick: retryAttempt }}
          >
            <Flex alignItems="center">
              <Text>{attempt.statusText}</Text>
            </Flex>
          </Alert>
        )}
        <InfoParagraph>
          {attempt.status === 'success' && !servers?.length && (
            <Text bold>No Results</Text>
          )}
          {attempt.status === 'success' && servers?.length > 0 && (
            <Box
              css={`
                position: relative;
                // negative margin to remove the padding we set
                // for the root container, b/c we want the list
                // to render flushed against the sides of box.
                margin-left: -${p => p.theme.space[3]}px;
                margin-right: -${p => p.theme.space[3]}px;
              `}
            >
              <UnhealthyServerList servers={unhealthyOrUnknownServers} />
            </Box>
          )}
          {attempt.status === 'processing' && (
            <Flex justifyContent="center">
              <Indicator />
            </Flex>
          )}
        </InfoParagraph>
      </Box>
      <div ref={setTrigger} />
    </>
  );
}

export function StatusInfoHeader({
  resource,
}: {
  resource: UnifiedResourceDefinition;
}) {
  if (resource.kind === 'db') {
    const icon = getDatabaseIconName(resource.protocol);
    return (
      <Flex gap={3}>
        <ResourceIcon name={icon} width="45px" height="45px" />
        <Box>
          <H2>{resource.name}</H2>
          {resource.type && (
            <Flex gap={1}>
              <DatabaseIcon size="small" color="text.slightlyMuted" />
              <SingleLineBox width="270px">
                <Text
                  typography="body3"
                  color="text.slightlyMuted"
                  title={resource.type}
                >
                  {resource.type}
                </Text>
              </SingleLineBox>
            </Flex>
          )}
        </Box>
      </Flex>
    );
  }
}

function ConnectionHeader({
  resource,
}: {
  resource: UnifiedResourceDefinition;
}) {
  if (resource.kind === 'db') {
    return (
      <Flex gap={2} my={3}>
        <WarningIcon size={16} />
        <H3>DB Connection Issue</H3>
      </Flex>
    );
  }
}

function StatusDescription({
  resource,
  numUnhealthyServers,
}: {
  resource: UnifiedResourceDefinition;
  numUnhealthyServers: number;
}) {
  if (resource.kind === 'db') {
    const health = resource.targetHealth;
    const startingWord = numUnhealthyServers > 1 ? 'Some' : 'A';
    const servicedWord = numUnhealthyServers
      ? pluralize(numUnhealthyServers, 'service')
      : 'service';
    switch (health.status) {
      case 'unhealthy':
        return (
          <>
            {startingWord} Teleport database {servicedWord} proxying access to
            this database cannot reach the database endpoint.
          </>
        );
      case 'unknown':
        return (
          <>
            {startingWord} Teleport database {servicedWord} proxying access to
            this database {numUnhealthyServers > 1 ? 'are' : 'is'} not running
            network health checks for the database endpoint. User connections
            will not be routed through affected Teleport database services as
            long as other database services report a healthy connection to the
            database.
          </>
        );
      default: // empty
        return (
          <>
            {startingWord} Teleport database {servicedWord} proxying access to
            this database requires upgrading to Teleport version{' '}
            <Mark>18.0.0</Mark> to run network health checks.
          </>
        );
    }
  }
}

function getTroubleShootingLink(resource: UnifiedResourceDefinition) {
  if (resource.kind == 'db') {
    return 'https://goteleport.com/docs/enroll-resources/database-access/getting-started/#troubleshooting';
  }
}

function getAffectedResourceKind(resource: UnifiedResourceDefinition) {
  switch (resource.kind) {
    case 'db':
      return 'database services';
  }
}

function UnhealthyServerList({ servers }: { servers: SharedResourceServer[] }) {
  const lastServerInList = servers.length - 1;
  return servers.map((server, index) => (
    <Flex
      gap={2}
      flexDirection="column"
      key={`${server.kind}/${server.hostId}`}
      css={`
        background-color: ${p => p.theme.colors.levels.sunken};
        padding: ${p => p.theme.space[3]}px;
        border-left: 4px solid
          ${p => p.theme.colors.interactive.solid.alert.default};
        ${index !== lastServerInList &&
        css`
          border-bottom: 1px solid ${p => p.theme.colors.spotBackground[1]};
        `}
      `}
    >
      <Text>
        <InfoField>Hostname:</InfoField> {server.hostname}
      </Text>
      <Text>
        <InfoField>UUID:</InfoField> {server.hostId}
      </Text>
      <Text>
        <InfoField>Reason:</InfoField> {server.targetHealth?.reason}
      </Text>
    </Flex>
  ));
}

const InfoField = styled.span`
  font-weight: bold;
`;
