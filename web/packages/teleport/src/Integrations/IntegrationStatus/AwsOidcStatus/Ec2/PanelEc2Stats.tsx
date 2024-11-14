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
import { Flex, H1, H2, H3, H4, H5, H6, Text, Box } from 'design';
import { SyncAlt } from 'design/Icon';
import { ToolTipInfo } from 'shared/components/ToolTip';
import * as Icons from 'design/Icon';

import { Integration } from 'teleport/services/integrations';

import {
  Panel,
  PanelTitle,
  CenteredSpaceBetweenFlex,
  CustomLabel,
  ErrorTooltip,
  InnerCard,
  GappedColumnFlex,
  PanelHeader,
  PanelLastSynced,
} from '../../Shared';

import { PanelIcon } from '../../getResourceIcon';

export function PanelEc2Stats({
  integration,
  route,
}: {
  integration: Integration;
  route: string;
}) {
  return (
    <Panel width="33%" as={InternalLink} to={route}>
      <Box>
        <PanelHeader>
          <PanelIcon type="ec2" />
          <H2>EC2</H2>
        </PanelHeader>
        <GappedColumnFlex>
          <CenteredSpaceBetweenFlex>
            <Text>Enrollment Rules</Text>
            <Text>12345</Text>
          </CenteredSpaceBetweenFlex>
          <CenteredSpaceBetweenFlex>
            <Text>Enrolled Instances</Text>
            <Text>12345</Text>
          </CenteredSpaceBetweenFlex>
          <CenteredSpaceBetweenFlex>
            <Text ml={4}>Failed Instances</Text>
            <Flex gap={2}>
              <Text>12345</Text>
              <Icons.Warning size="large" color="error.main" />
            </Flex>
          </CenteredSpaceBetweenFlex>
        </GappedColumnFlex>
      </Box>
      <PanelLastSynced />
    </Panel>
  );
}
