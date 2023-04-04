/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
                {props.loggedInUser.rolesList.map(role => (
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
  background: ${props => props.theme.colors.levels.elevated};
  height: 1px;
`;
