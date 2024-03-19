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
import styled from 'styled-components';
import { Box, Flex, Text, Label } from 'design';

import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { LoggedInUser } from 'teleterm/services/tshd/types';

import { IdentityRootCluster } from '../useIdentity';

import { IdentityListItem } from './IdentityListItem';
import { AddNewClusterItem } from './AddNewClusterItem';

interface IdentityListProps {
  loggedInUser: LoggedInUser;
  clusters: IdentityRootCluster[];

  onSelectCluster(clusterUri: string): void;

  onAddCluster(): void;

  onLogout(clusterUri: string): void;
}

export function IdentityList(props: IdentityListProps) {
  return (
    <Box minWidth="200px">
      {props.loggedInUser && (
        <>
          <Flex px={3} pt={2} pb={2} justifyContent="space-between">
            <Box>
              <Text bold>{props.loggedInUser.name}</Text>
              <Flex flexWrap="wrap" gap={1}>
                {props.loggedInUser.roles.map(role => (
                  <Label key={role} kind="secondary">
                    {role}
                  </Label>
                ))}
              </Flex>
            </Box>
          </Flex>
          <Separator />
        </>
      )}
      <KeyboardArrowsNavigation>
        {focusGrabber}
        <Box>
          {props.clusters.map((i, index) => (
            <IdentityListItem
              key={i.uri}
              index={index}
              isSelected={i.active}
              userName={i.userName}
              clusterName={i.clusterName}
              onSelect={() => props.onSelectCluster(i.uri)}
              onLogout={() => props.onLogout(i.uri)}
            />
          ))}
        </Box>
        <Separator />
        <Box>
          <AddNewClusterItem
            index={props.clusters.length + 1}
            onClick={props.onAddCluster}
          />
        </Box>
      </KeyboardArrowsNavigation>
    </Box>
  );
}

// Hack - for some reason xterm.js doesn't allow moving a focus to the Identity popover
// when it is focused using element.focus(). Moreover, it looks like this solution has a benefit
// of returning the focus to the previously focused element when popover is closed.
const focusGrabber = (
  <input
    style={{
      opacity: 0,
      position: 'absolute',
      height: 0,
      zIndex: -1,
    }}
    autoFocus={true}
  />
);

const Separator = styled.div`
  background: ${props => props.theme.colors.spotBackground[1]};
  height: 1px;
`;
