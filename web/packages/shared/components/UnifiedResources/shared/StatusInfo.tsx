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

import { type JSX } from 'react';
import styled, { css } from 'styled-components';

import { Box, ButtonBorder, Indicator, Text } from 'design';
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
  InfoGuideConfig,
  InfoParagraph,
  InfoTitle,
} from 'shared/components/SlidingSidePanel/InfoGuide';
import { resourceStatusPanelWidth } from 'shared/components/SlidingSidePanel/InfoGuide/const';
import { useInfiniteScroll } from 'shared/hooks';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { pluralize } from 'shared/utils/text';

import {
  DatabaseServer,
  ResourceHealthStatus,
  SharedResourceServer,
  UnifiedResourceDefinition,
} from '../types';
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

  const unhealthyOrUnknownServers = servers?.filter(
    s => s.targetHealth?.status !== 'healthy'
  );

  return (
    <>
      <Box>
        <ConnectionHeader resource={resource} />
        {unhealthyOrUnknownServers.length > 0 && (
          <InfoParagraph>
            <StatusDescription
              fetchedServers={unhealthyOrUnknownServers}
              resource={resource}
            />
          </InfoParagraph>
        )}
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
            // Refresh might be required if all health checks passed
            // while unified resources page has been stale.
            <Text bold>No Results. Try refreshing the page.</Text>
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
  fetchedServers,
  resource,
}: {
  fetchedServers: DatabaseServer[];
  resource: UnifiedResourceDefinition;
}) {
  if (resource.kind === 'db') {
    const unhealthyServers = fetchedServers.filter(
      s => s.targetHealth?.status === 'unhealthy'
    ).length;
    const unknownHealthServers = fetchedServers.filter(
      s => s.targetHealth?.status === 'unknown'
    ).length;

    if (unhealthyServers && unknownHealthServers) {
      return (
        <StyledUl>
          <li>{unhealthyStatus(unhealthyServers)}</li>
          <li>{unknownStatus(unknownHealthServers)}</li>
        </StyledUl>
      );
    }

    if (unhealthyServers) {
      return <>{unhealthyStatus(unhealthyServers)}</>;
    }

    return <>{unknownStatus(unknownHealthServers)}</>;
  }
}

function unhealthyStatus(numServers: number) {
  const startingWord = numServers > 1 ? 'Some' : 'A';
  const serviceWord = numServers ? pluralize(numServers, 'service') : 'service';
  return (
    <>
      {startingWord} Teleport database {serviceWord} proxying access to this
      database cannot reach the database endpoint.
    </>
  );
}

function unknownStatus(numServers: number) {
  const startingWord = numServers > 1 ? 'Some' : 'A';
  const serviceWord = numServers ? pluralize(numServers, 'service') : 'service';
  return (
    <>
      {startingWord} Teleport database {serviceWord} proxying access to this
      database {numServers > 1 ? 'are' : 'is'} not running network health checks
      for the database endpoint. User connections will not be routed through
      affected Teleport database services as long as other database services
      report a healthy connection to the database.
    </>
  );
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
        <InfoField>Status:</InfoField>{' '}
        {server.targetHealth?.status || 'unknown'}
      </Text>
      {server.targetHealth?.message && (
        <Text>
          <InfoField>Message:</InfoField> {server.targetHealth.message}
        </Text>
      )}
      {server.targetHealth?.error && (
        <Text>
          <InfoField>Error:</InfoField> {server.targetHealth.error}
        </Text>
      )}
    </Flex>
  ));
}

const InfoField = styled.span`
  font-weight: bold;
`;

/**
 * Returns a unique id by appending the resource kind with
 * their name/id (for most resources their id is the "name" field,
 * other resources does not have name field, but an "id" field).
 */
export function getResourceId(resource: UnifiedResourceDefinition) {
  const kind = resource.kind;
  let id;
  if (kind === 'node' || kind === 'git_server') {
    id = resource.id;
  } else {
    id = resource.name;
  }

  return `${kind}/${id}`;
}

export function openStatusInfoPanel({
  resource,
  setInfoGuideConfig,
  guide,
  isEnterprise = false,
}: {
  resource: UnifiedResourceDefinition;
  setInfoGuideConfig: (cfg: InfoGuideConfig) => void;
  guide: JSX.Element;
  isEnterprise?: boolean;
}) {
  if (resource.kind === 'db') {
    setInfoGuideConfig({
      guide,
      id: getResourceId(resource),
      title: <StatusInfoHeader resource={resource} />,
      panelWidth: resourceStatusPanelWidth,
      viewHasOwnSidePanel: isEnterprise,
    });
  }
}

/**
 * Returns true if any status is unhealthy or if there are a mix of different
 * health statuses.
 */
export function shouldWarnResourceStatus(
  status: ResourceHealthStatus
): boolean {
  return status === 'mixed' || status === 'unhealthy';
}

export const StyledUl = styled.ul`
  margin: 0;
  padding-left: ${p => p.theme.space[4]}px;
  padding-bottom: ${p => p.theme.space[1]}px;
`;
