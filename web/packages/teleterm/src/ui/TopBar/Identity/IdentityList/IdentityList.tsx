import React from 'react';
import { KeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import styled from 'styled-components';
import { Box, Flex, Text } from 'design';
import { LoggedInUser } from 'teleterm/services/tshd/types';
import { IdentityListItem } from './IdentityListItem';
import { AddNewClusterItem } from './AddNewClusterItem';
import { IdentityRootCluster } from '../useIdentity';

interface IdentityListProps {
  loggedInUser: LoggedInUser;
  clusters: IdentityRootCluster[];

  onSelectCluster(clusterUri: string): void;

  onAddCluster(): void;

  onLogout(clusterUri: string): void;
}

export function IdentityList(props: IdentityListProps) {
  const roles = props.loggedInUser.rolesList.join(', ');

  return (
    <Box minWidth="200px" pt="12px">
      {props.loggedInUser && (
        <>
          <Flex px="24px" pb={2} justifyContent="space-between">
            <Box>
              <Text bold>{props.loggedInUser.name}</Text>
              <Text typography="body2" color="text.secondary">
                {roles}
              </Text>
            </Box>
          </Flex>
          <Separator />
        </>
      )}
      <KeyboardArrowsNavigation>
        {focusGrabber}
        <Box px={'12px'}>
          {props.clusters.map((i, index) => (
            <IdentityListItem
              key={i.uri}
              index={index}
              isSelected={i.active}
              userName={i.userName}
              clusterName={i.clusterName}
              isSyncing={i.clusterSyncStatus}
              onSelect={() => props.onSelectCluster(i.uri)}
              onLogout={() => props.onLogout(i.uri)}
            />
          ))}
        </Box>
        <Separator />
        <Box px={'12px'}>
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
  background: ${props => props.theme.colors.primary.lighter};
  margin: 0 16px;
  height: 1px;
`;
