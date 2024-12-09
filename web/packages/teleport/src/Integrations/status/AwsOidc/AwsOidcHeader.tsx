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
import { Link as InternalLink } from 'react-router-dom';

import { ButtonIcon, Flex, Label, Text } from 'design';
import { ArrowLeft } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import { getStatusAndLabel } from 'teleport/Integrations/helpers';
import { Integration } from 'teleport/services/integrations';

export function AwsOidcHeader({ integration }: { integration: Integration }) {
  const { status, labelKind } = getStatusAndLabel(integration);
  return (
    <Flex alignItems="center">
      <HoverTooltip position="bottom" tipContent="Back to Integrations">
        <ButtonIcon
          as={InternalLink}
          to={cfg.routes.integrations}
          aria-label="back"
        >
          <ArrowLeft size="medium" />
        </ButtonIcon>
      </HoverTooltip>
      <Text bold fontSize={6} mr={2}>
        {integration.name}
      </Text>
      <Label kind={labelKind} aria-label="status" px={3}>
        {status}
      </Label>
    </Flex>
  );
}
