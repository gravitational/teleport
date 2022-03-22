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
  return (
    <>
      <Flex px={'24px'} pb={2} justifyContent="space-between">
        <Box>
          <Text bold>{props.loggedInUser?.name}</Text>
          <Text typography="body2" color="text.secondary">
            {props.loggedInUser?.rolesList?.join(', ')}
          </Text>
        </Box>
      </Flex>
      <Separator />
      <KeyboardArrowsNavigation>
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
    </>
  );
}

const Separator = styled.div`
  background: ${props => props.theme.colors.primary.lighter};
  margin: 0 16px;
  height: 1px;
`;
