/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
import { Text, Flex } from 'design';

import { ToolTipInfo } from './ToolTip';

export default {
  title: 'Shared/ToolTip',
};

export const ShortContent = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <ToolTipInfo>"some popover content"</ToolTipInfo>
  </>
);

export const LongContent = () => (
  <Flex alignItems="center">
    <Text mr={1}>Hover the icon</Text>
    <ToolTipInfo>
      <Text>
        Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod
        tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
        veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea
        commodo consequat.
      </Text>
      <Text mt={1}>
        Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
        dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non
        proident, sunt in culpa qui officia deserunt mollit anim id est laborum.
      </Text>
    </ToolTipInfo>
  </Flex>
);

export const WithMutedIconColor = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <ToolTipInfo muteIconColor>"some popover content"</ToolTipInfo>
  </>
);

export const WithKindWarning = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <ToolTipInfo kind="warning">"some popover content"</ToolTipInfo>
  </>
);

export const WithKindError = () => (
  <>
    <span css={{ marginRight: '4px', verticalAlign: 'middle' }}>
      Hover the icon
    </span>
    <ToolTipInfo kind="error">"some popover content"</ToolTipInfo>
  </>
);
