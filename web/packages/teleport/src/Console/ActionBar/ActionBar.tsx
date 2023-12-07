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

import { NavLink } from 'react-router-dom';
import { MenuIcon, MenuItem, MenuItemIcon } from 'shared/components/MenuAction';
import { LatencyDiagnostic } from 'shared/components/LatencyDiagnostic';

import * as Icons from 'design/Icon';
import { Flex, ButtonPrimary } from 'design';

import cfg from 'teleport/config';
import { DocumentSsh } from 'teleport/Console/stores';

export default function ActionBar(props: Props) {
  return (
    <Flex alignItems="center">
      {props.latencyIndicator.isVisible && (
        <LatencyDiagnostic latency={props.latencyIndicator.latency} />
      )}
      <MenuIcon
        buttonIconProps={{ mr: 2, ml: 2, size: 0, style: { fontSize: '16px' } }}
        menuProps={menuProps}
      >
        <MenuItem as={NavLink} to={cfg.routes.root}>
          <MenuItemIcon as={Icons.Home} mr="2" size="medium" />
          Home
        </MenuItem>
        <MenuItem>
          <ButtonPrimary my={3} block onClick={props.onLogout}>
            Sign Out
          </ButtonPrimary>
        </MenuItem>
      </MenuIcon>
    </Flex>
  );
}

type Props = {
  latencyIndicator:
    | { isVisible: true; latency: DocumentSsh['latency'] }
    | {
        isVisible: false;
      };
  onLogout: VoidFunction;
};

const menuListCss = () => `
  width: 250px;
`;

const menuProps = {
  menuListCss,
} as const;
