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
import { Flex, Text } from 'design';
import { SyncAlt } from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { Integration } from 'teleport/services/integrations';

import {
  Panel,
  PanelTitle,
  CenteredSpaceBetweenFlex,
  CustomLabel,
  ErrorTooltip,
  InnerCard,
} from '../Shared';

import { PanelIcon } from '../getResourceIcon';

export function PanelAwsConsoleCliAccess({
  integration,
}: {
  integration: Integration;
}) {
  return (
    <Panel width="65%">
      <PanelTitle>AWS Console and CLI Access</PanelTitle>
      <Text>Allows to create new app resources, to access AWS account.</Text>
    </Panel>
  );
}
