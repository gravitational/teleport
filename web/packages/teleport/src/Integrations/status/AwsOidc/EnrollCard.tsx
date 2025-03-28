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
import { Link as InternalLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, Card, Flex, H2, H3, P2, ResourceIcon } from 'design';
import * as Icons from 'design/Icon';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';

export function EnrollCard({
  resource,
  item,
}: {
  resource: AwsResource;
  item: string;
}) {
  return (
    <Enroll
      data-testid={`${resource}-enroll`}
      as={InternalLink}
      to={{
        pathname: cfg.routes.discover,
        state: { searchKeywords: resource },
      }}
    >
      <Flex flexDirection="column" justifyContent="space-between" height="100%">
        <Box>
          <Flex alignItems="center">
            <ResourceIcon name={resource} mr={2} width="32px" height="32px" />
            <H2>{resource.toUpperCase()}</H2>
          </Flex>
          <P2 mb={2}>
            Discover and enroll {resource.toUpperCase()} {item}
          </P2>
        </Box>
        <Flex alignItems="center" gap={2}>
          <H3>Enroll {resource.toUpperCase()}</H3>
          <Icons.ArrowForward />
        </Flex>
      </Flex>
    </Enroll>
  );
}

const Enroll = styled(Card)`
  width: 33%;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: ${props => props.theme.space[3]}px;
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
