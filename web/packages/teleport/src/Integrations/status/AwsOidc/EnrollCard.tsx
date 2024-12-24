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

import { useHistory } from 'react-router';
import styled from 'styled-components';

import { ButtonBorder, Card, Flex, H2, ResourceIcon } from 'design';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';

type EnrollCardProps = {
  resource: AwsResource;
};

export function EnrollCard({ resource }: EnrollCardProps) {
  const history = useHistory();

  const handleClick = () => {
    history.push({
      pathname: cfg.routes.discover,
      state: { searchKeywords: resource },
    });
  };

  // todo (michellescripts) update enroll design once ready
  return (
    <Enroll data-testid={`${resource}-enroll`}>
      <Flex flexDirection="column" gap={4}>
        <Flex alignItems="center" mb={2}>
          <ResourceIcon name={resource} mr={2} width="32px" height="32px" />
          <H2>{resource.toUpperCase()}</H2>
        </Flex>
        <ButtonBorder size="large" onClick={handleClick}>
          Enroll {resource.toUpperCase()}
        </ButtonBorder>
      </Flex>
    </Enroll>
  );
}

const Enroll = styled(Card)`
  width: 33%;
  background-color: ${props => props.theme.colors.levels.surface};
  padding: 12px;
  border-radius: ${props => props.theme.radii[2]}px;
  border: ${props => `1px solid ${props.theme.colors.levels.surface}`};

  &:hover {
    background-color: ${props => props.theme.colors.levels.elevated};
    box-shadow: ${({ theme }) => theme.boxShadow[2]};
  }
`;
