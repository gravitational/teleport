/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React from 'react';
import styled from 'styled-components';
import { useParams, browserHistory } from 'react-router';
import { Link as InternalLink, useHistory } from 'react-router-dom';

import { Flex, ButtonIcon, Text, Label } from 'design';
import { ArrowLeft, ChevronRight } from 'design/Icon';
import { HoverTooltip } from 'shared/components/ToolTip';

import { AttemptStatus } from 'shared/hooks/useAsync';

import { Integration } from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { getStatusAndLabel } from '../getStatusAndLabel';
import { AwsResourceKind } from '../Shared';

export function AwsOidcHeader({
  integration,
  attemptStatus,
  secondaryHeader,
}: {
  integration: Integration | null;
  attemptStatus: AttemptStatus;
  secondaryHeader?: AwsResourceKind | 'tasks';
}) {
  const history = useHistory();
  const { name: integrationNameFromParam } = useParams<{ name: string }>();

  const BackButton = (
    <HoverTooltip tipContent="Back">
      <ButtonIcon
        as={InternalLink}
        to={cfg.routes.integrations}
        onClick={() => history.goBack()}
      >
        <ArrowLeft size="medium" />
      </ButtonIcon>
    </HoverTooltip>
  );

  if (attemptStatus === 'success' && integration) {
    const { status, labelKind } = getStatusAndLabel(integration);
    return (
      <Flex alignItems="center" gap={3}>
        {BackButton}
        <Flex alignItems="center" gap={1}>
          <Text bold fontSize={6}>
            {integration?.name}
          </Text>
          {secondaryHeader && (
            <>
              <ChevronRight />
              <Text bold fontSize={6}>
                {secondaryHeader === 'tasks'
                  ? 'Pending Tasks'
                  : secondaryHeader.toUpperCase()}
              </Text>
            </>
          )}
        </Flex>
        <Label kind={labelKind}>{status}</Label>
      </Flex>
    );
  }

  // All other attempt states.
  return (
    <Flex>
      <Flex alignItems="center" gap={1}>
        {BackButton}
        <Text bold fontSize={6} mr={2}>
          {integrationNameFromParam}
        </Text>
      </Flex>
    </Flex>
  );
}

export const SpaceBetweenFlexedHeader = styled(Flex)`
  justify-content: space-between;
  margin-bottom: ${p => p.theme.space[5]}px;
  margin-top: ${p => p.theme.space[5]}px;
`;
